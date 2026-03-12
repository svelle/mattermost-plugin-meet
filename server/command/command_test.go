package command

import (
	"errors"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-meet/server/pluginerrors"
)

type mockMeetingStarter struct {
	configured     bool
	connected      bool
	isAdmin        bool
	connectURL     string
	configureURL   string
	startErr       error
	connectedErr   error
	adminErr       error
	startedMeeting struct {
		userID    string
		channelID string
		topic     string
	}
}

func (m *mockMeetingStarter) StartMeeting(userID, channelID, topic string) error {
	m.startedMeeting.userID = userID
	m.startedMeeting.channelID = channelID
	m.startedMeeting.topic = topic
	return m.startErr
}

func (m *mockMeetingStarter) GetConnectURL() string         { return m.connectURL }
func (m *mockMeetingStarter) IsPluginConfigured() bool      { return m.configured }
func (m *mockMeetingStarter) GetPluginConfigureURL() string { return m.configureURL }

func (m *mockMeetingStarter) IsUserConnected(_ string) (bool, error) {
	return m.connected, m.connectedErr
}

func (m *mockMeetingStarter) IsUserAdmin(_ string) (bool, error) {
	return m.isAdmin, m.adminErr
}

func TestHandle_UnknownCommand(t *testing.T) {
	mock := &mockMeetingStarter{configured: true, connected: true}
	handler := &Handler{meetingStarter: mock}

	resp, err := handler.Handle(&model.CommandArgs{Command: "/unknown"})
	require.NoError(t, err)
	assert.Contains(t, resp.Text, "Unknown command")
}

func TestHandle_EmptyCommand(t *testing.T) {
	mock := &mockMeetingStarter{configured: true, connected: true}
	handler := &Handler{meetingStarter: mock}

	resp, err := handler.Handle(&model.CommandArgs{Command: ""})
	require.NoError(t, err)
	assert.Equal(t, "Empty command", resp.Text)
}

func TestExecuteMeetCommand_NotConfigured_Admin(t *testing.T) {
	mock := &mockMeetingStarter{
		configured:   false,
		isAdmin:      true,
		configureURL: "http://localhost/admin",
	}
	handler := &Handler{meetingStarter: mock}

	resp, err := handler.Handle(&model.CommandArgs{
		Command:   "/meet",
		UserId:    "user1",
		ChannelId: "chan1",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Text, "not configured")
	assert.Contains(t, resp.Text, "http://localhost/admin")
	assert.Equal(t, model.CommandResponseTypeEphemeral, resp.ResponseType)
}

func TestExecuteMeetCommand_NotConfigured_NonAdmin(t *testing.T) {
	mock := &mockMeetingStarter{configured: false, isAdmin: false}
	handler := &Handler{meetingStarter: mock}

	resp, err := handler.Handle(&model.CommandArgs{
		Command:   "/meet",
		UserId:    "user1",
		ChannelId: "chan1",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Text, "not configured")
	assert.Contains(t, resp.Text, "system administrator")
}

func TestExecuteMeetCommand_NotConfigured_AdminCheckError(t *testing.T) {
	mock := &mockMeetingStarter{
		configured: false,
		adminErr:   errors.New("api error"),
	}
	handler := &Handler{meetingStarter: mock}

	resp, err := handler.Handle(&model.CommandArgs{
		Command:   "/meet",
		UserId:    "user1",
		ChannelId: "chan1",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Text, "Failed to check permissions")
}

func TestExecuteMeetCommand_NotConnected(t *testing.T) {
	mock := &mockMeetingStarter{
		configured: true,
		connected:  false,
		connectURL: "http://localhost/connect",
	}
	handler := &Handler{meetingStarter: mock}

	resp, err := handler.Handle(&model.CommandArgs{
		Command:   "/meet",
		UserId:    "user1",
		ChannelId: "chan1",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Text, "connect your Google account")
	assert.Contains(t, resp.Text, "http://localhost/connect")
}

func TestExecuteMeetCommand_ConnectionCheckError(t *testing.T) {
	mock := &mockMeetingStarter{
		configured:   true,
		connectedErr: errors.New("kv error"),
	}
	handler := &Handler{meetingStarter: mock}

	resp, err := handler.Handle(&model.CommandArgs{
		Command:   "/meet",
		UserId:    "user1",
		ChannelId: "chan1",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Text, "Failed to check Google connection status")
}

func TestExecuteMeetCommand_Success(t *testing.T) {
	mock := &mockMeetingStarter{configured: true, connected: true}
	handler := &Handler{meetingStarter: mock}

	resp, err := handler.Handle(&model.CommandArgs{
		Command:   "/meet",
		UserId:    "user1",
		ChannelId: "chan1",
	})
	require.NoError(t, err)
	assert.Equal(t, "", resp.Text)
	assert.Equal(t, "user1", mock.startedMeeting.userID)
	assert.Equal(t, "chan1", mock.startedMeeting.channelID)
	assert.Equal(t, "", mock.startedMeeting.topic)
}

func TestExecuteMeetCommand_SuccessWithTopic(t *testing.T) {
	mock := &mockMeetingStarter{configured: true, connected: true}
	handler := &Handler{meetingStarter: mock}

	resp, err := handler.Handle(&model.CommandArgs{
		Command:   "/meet Sprint Planning",
		UserId:    "user1",
		ChannelId: "chan1",
	})
	require.NoError(t, err)
	assert.Equal(t, "", resp.Text)
	assert.Equal(t, "Sprint Planning", mock.startedMeeting.topic)
}

func TestExecuteMeetCommand_NeedsReconnect(t *testing.T) {
	mock := &mockMeetingStarter{
		configured: true,
		connected:  true,
		startErr:   pluginerrors.ErrNeedsReconnect,
		connectURL: "http://localhost/connect",
	}
	handler := &Handler{meetingStarter: mock}

	resp, err := handler.Handle(&model.CommandArgs{
		Command:   "/meet",
		UserId:    "user1",
		ChannelId: "chan1",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Text, "reconnected")
	assert.Contains(t, resp.Text, "http://localhost/connect")
}

func TestExecuteMeetCommand_StartMeetingError(t *testing.T) {
	mock := &mockMeetingStarter{
		configured: true,
		connected:  true,
		startErr:   errors.New("some internal error"),
	}
	handler := &Handler{meetingStarter: mock}

	resp, err := handler.Handle(&model.CommandArgs{
		Command:   "/meet",
		UserId:    "user1",
		ChannelId: "chan1",
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Text, "Failed to create meeting")
	// Must NOT contain the internal error message
	assert.NotContains(t, resp.Text, "some internal error")
}
