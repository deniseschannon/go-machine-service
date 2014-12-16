package handlers

import (
	"github.com/fsouza/go-dockerclient"
	"github.com/rancherio/go-machine-service/events"
	tu "github.com/rancherio/go-machine-service/test_utils"
	"github.com/rancherio/go-machine-service/utils"
	"log"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestCleanSanity(t *testing.T) {
	log.Println("Handler sanity test passed")
}

func TestMachineHandlers(t *testing.T) {
	// TODO Add env based switch to decide what type of machine to create
	// ie, vbox for local vs google compute CI
	resourceId := "test-" + strconv.FormatInt(time.Now().Unix(), 10)
	event := &events.Event{
		ResourceId: resourceId,
		Id:         "event-id",
		ReplyTo:    "reply-to-id",
	}

	mockApiClient := &tu.MockApiClient{}
	mockPhysHost, _ := mockApiClient.GetPhysicalHost(event.ResourceId)

	replyCalled := false
	replyEventHandler := func(replyEvent *events.ReplyEvent) {
		replyCalled = true
	}

	CreateMachine(event, replyEventHandler, mockApiClient)
	if !replyCalled {
		tu.FailNowStackf(t, "Reply not called for event [%v]", event.Id)
	}

	// Idempotent check. Should rerun and reply without error
	replyCalled = false
	CreateMachine(event, replyEventHandler, mockApiClient)
	if !replyCalled {
		tu.FailNowStackf(t, "Idempotent check failed for CreateMachine. Event: %v", event.Id)
	}

	// TODO Converting name here is cheating a bit. Should find a way to remove this inside knowlege
	machineName := convertToName(mockPhysHost.ExternalId)

	// and test activating that machine
	andActivateMachine(resourceId, machineName, t)

	// and t4est purging that machine
	andPurgeMachine(resourceId, machineName, t)
}

func andActivateMachine(resourceId string, machineName string, t *testing.T) {
	deleteContainer(machineName, "rancher-agent-bootstrap", t)

	if url := utils.GetRancherUrl(true); url == "" {
		// TODO Make sure CI sets it explicitly
		os.Setenv("CATTLE_URL_FOR_AGENT", "http://10.0.2.2:8080")
	}

	mockApiClient := &tu.MockApiClient{}

	event := &events.Event{
		ResourceId: resourceId,
		Id:         "event-id",
		ReplyTo:    "reply-to-id",
	}

	replyCalled := false
	replyEventHandler := func(replyEvent *events.ReplyEvent) {
		replyCalled = true
		if replyEvent.Name != event.ReplyTo {
			tu.FailNowStackf(t, "ReplyTo not set properly.")
		}
	}

	ActivateMachine(event, replyEventHandler, mockApiClient)
	if !replyCalled {
		tu.FailNowStackf(t, "Reply not called for event [%v]", event.Id)
	}

	// Idempotent check. Should run and reply again without breaking.
	replyCalled = false
	ActivateMachine(event, replyEventHandler, mockApiClient)
	if !replyCalled {
		tu.FailNowStackf(t, "Idempotent check failed for ActivateMachine. Event: %v", event.Id)
	}
}

func andPurgeMachine(resourceId string, machineName string, t *testing.T) {
	event := &events.Event{
		ResourceId: resourceId,
		Id:         "event-id",
		ReplyTo:    "reply-to-id",
	}

	mockApiClient := &tu.MockApiClient{}

	replyCalled := false
	replyEventHandler := func(replyEvent *events.ReplyEvent) {
		replyCalled = true
	}

	PurgeMachine(event, replyEventHandler, mockApiClient)
	if !replyCalled {
		tu.FailNowStackf(t, "Reply not called for event [%v]", event.Id)
	}

	// Idempotent check. Should run and reply again without breaking.
	replyCalled = false
	PurgeMachine(event, replyEventHandler, mockApiClient)
	if !replyCalled {
		tu.FailNowStackf(t, "Idempotent check failed for PurgeMachine. Event: %v", event.Id)
	}
}

func deleteContainer(machineName string, containerName string, t *testing.T) {
	client, err := utils.GetDockerClient(machineName)
	tu.CheckError(err, t)

	containers, err := utils.FindContainersByNames(client, containerName)
	tu.CheckError(err, t)

	removeOpts := docker.RemoveContainerOptions{
		RemoveVolumes: true,
		Force:         true,
	}

	for _, container := range containers {
		removeOpts.ID = container.ID
		err := client.RemoveContainer(removeOpts)
		tu.CheckError(err, t)
	}
}
