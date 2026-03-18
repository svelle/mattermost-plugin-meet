package command

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

// MeetingStarter is the interface the command handler uses to start meetings.
type MeetingStarter interface {
	StartMeeting(userID, channelID, topic string) error
	GetConnectURL() string
	DisconnectUser(userID string) error
	IsUserConnected(userID string) (bool, error)
	IsPluginConfigured() bool
	IsUserAdmin(userID string) (bool, error)
	GetPluginConfigureURL() string
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
	autocompleteData := model.NewAutocompleteData(meetCommandTrigger, "", "Google Meet commands")
	autocompleteData.AddCommand(model.NewAutocompleteData("start", "[topic]", "Start a Google Meet meeting with an optional topic"))
	autocompleteData.AddCommand(model.NewAutocompleteData("connect", "", "Connect or reconnect your Google account"))
	autocompleteData.AddCommand(model.NewAutocompleteData("disconnect", "", "Disconnect your Google account"))
	autocompleteData.AddCommand(model.NewAutocompleteData("help", "", "Show Google Meet command help"))

	err := client.SlashCommand.Register(&model.Command{
		Trigger:          meetCommandTrigger,
		AutoComplete:     true,
		AutoCompleteDesc: "Google Meet commands",
		AutoCompleteHint: "help | start [topic] | connect | disconnect",
		AutocompleteData: autocompleteData,
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
		if len(fields) == 1 {
			return c.executeMeetHelpCommand(), nil
		}
		switch fields[1] {
		case "help":
			return c.executeMeetHelpCommand(), nil
		case "start":
			return c.executeMeetStartCommand(args, fields[2:]), nil
		case "connect":
			return c.executeMeetConnectCommand(args), nil
		case "disconnect":
			return c.executeMeetDisconnectCommand(args), nil
		default:
			return c.executeMeetHelpCommand(), nil
		}
	default:
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unknown command: %s", args.Command),
		}, nil
	}
}

func (c *Handler) executeMeetHelpCommand() *model.CommandResponse {
	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text: strings.Join([]string{
			"Google Meet commands:",
			"- `/meet start [topic]` starts a meeting with an optional topic.",
			"- `/meet connect` opens the Google account connection flow.",
			"- `/meet disconnect` removes your saved Google connection.",
			"- `/meet help` shows this help message.",
		}, "\n"),
	}
}

func (c *Handler) pluginNotConfiguredResponse(userID string) *model.CommandResponse {
	if !c.meetingStarter.IsPluginConfigured() {
		isAdmin, err := c.meetingStarter.IsUserAdmin(userID)
		if err != nil {
			return &model.CommandResponse{
				ResponseType: model.CommandResponseTypeEphemeral,
				Text:         "Failed to check permissions. Please try again.",
			}
		}
		if isAdmin {
			configURL := c.meetingStarter.GetPluginConfigureURL()
			if configURL == "" {
				return &model.CommandResponse{
					ResponseType: model.CommandResponseTypeEphemeral,
					Text:         "The Google Meet plugin is not configured. Mattermost Site URL must be configured before the System Console link is available.",
				}
			}
			return &model.CommandResponse{
				ResponseType: model.CommandResponseTypeEphemeral,
				Text:         fmt.Sprintf("The Google Meet plugin is not configured. [Configure it in the System Console](%s).", configURL),
			}
		}
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "The Google Meet plugin is not configured. Please contact your system administrator.",
		}
	}

	return nil
}

func (c *Handler) executeMeetConnectCommand(args *model.CommandArgs) *model.CommandResponse {
	if resp := c.pluginNotConfiguredResponse(args.UserId); resp != nil {
		return resp
	}

	connectURL := c.meetingStarter.GetConnectURL()
	if connectURL == "" {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "The Google account connection link is unavailable. Please contact your system administrator.",
		}
	}

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         fmt.Sprintf("Connect or reconnect your Google account. [Click here to continue](%s).", connectURL),
	}
}

func (c *Handler) executeMeetDisconnectCommand(args *model.CommandArgs) *model.CommandResponse {
	if err := c.meetingStarter.DisconnectUser(args.UserId); err != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Failed to disconnect your Google account. Please try again.",
		}
	}

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		Text:         "Your Google account has been disconnected.",
	}
}

func (c *Handler) executeMeetStartCommand(args *model.CommandArgs, topicFields []string) *model.CommandResponse {
	if resp := c.pluginNotConfiguredResponse(args.UserId); resp != nil {
		return resp
	}

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

	topic := strings.Join(topicFields, " ")

	if err := c.meetingStarter.StartMeeting(args.UserId, args.ChannelId, topic); err != nil {
		if errors.Is(err, ErrNeedsReconnect) {
			connectURL := c.meetingStarter.GetConnectURL()
			return &model.CommandResponse{
				ResponseType: model.CommandResponseTypeEphemeral,
				Text:         fmt.Sprintf("Your Google account needs to be reconnected. [Click here to reconnect](%s).", connectURL),
			}
		}

		if errors.Is(err, ErrPublicChannelRestricted) {
			return &model.CommandResponse{
				ResponseType: model.CommandResponseTypeEphemeral,
				Text:         "Meeting creation is restricted in public channels. Try a private channel or direct message instead.",
			}
		}
		// NewCommandHandler always sets c.client, but tests may construct Handler directly.
		if c.client != nil {
			c.client.Log.Error("Failed to create meeting", "user_id", args.UserId, "error", err.Error())
		}
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Failed to create meeting. Please try again or check the server logs.",
		}
	}

	return &model.CommandResponse{}
}
