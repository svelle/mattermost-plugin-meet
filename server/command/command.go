package command

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

// MeetingStarter is the interface the command handler uses to start meetings.
type MeetingStarter interface {
	StartMeeting(userID, channelID, topic string) error
	GetConnectURL() string
	IsUserConnected(userID string) (bool, error)
}

type Handler struct {
	client         *pluginapi.Client
	meetingStarter MeetingStarter
}

type Command interface {
	Handle(args *model.CommandArgs) (*model.CommandResponse, error)
}

const meetCommandTrigger = "meet"

func NewCommandHandler(client *pluginapi.Client, meetingStarter MeetingStarter) Command {
	err := client.SlashCommand.Register(&model.Command{
		Trigger:          meetCommandTrigger,
		AutoComplete:     true,
		AutoCompleteDesc: "Start a Google Meet meeting",
		AutoCompleteHint: "[topic]",
		AutocompleteData: model.NewAutocompleteData(meetCommandTrigger, "[topic]", "Start a Google Meet meeting with an optional topic"),
	})
	if err != nil {
		client.Log.Error("Failed to register command", "error", err)
	}
	return &Handler{
		client:         client,
		meetingStarter: meetingStarter,
	}
}

func (c *Handler) Handle(args *model.CommandArgs) (*model.CommandResponse, error) {
	fields := strings.Fields(args.Command)
	if len(fields) == 0 {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Empty command",
		}, nil
	}
	trigger := strings.TrimPrefix(fields[0], "/")
	switch trigger {
	case meetCommandTrigger:
		return c.executeMeetCommand(args), nil
	default:
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unknown command: %s", args.Command),
		}, nil
	}
}

func (c *Handler) executeMeetCommand(args *model.CommandArgs) *model.CommandResponse {
	connected, err := c.meetingStarter.IsUserConnected(args.UserId)
	if err != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Failed to check Google connection status. Please try again.",
		}
	}

	if !connected {
		connectURL := c.meetingStarter.GetConnectURL()
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("You need to connect your Google account first. [Click here to connect](%s).", connectURL),
		}
	}

	// Extract topic from command arguments
	topic := ""
	fields := strings.Fields(args.Command)
	if len(fields) > 1 {
		topic = strings.Join(fields[1:], " ")
	}

	if err := c.meetingStarter.StartMeeting(args.UserId, args.ChannelId, topic); err != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Failed to create meeting: %s", err.Error()),
		}
	}

	return &model.CommandResponse{}, nil
}
