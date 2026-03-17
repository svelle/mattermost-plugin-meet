# Mattermost Google Meet Plugin

Start and join Google Meet meetings from Mattermost.

This plugin adds `/meet` slash commands and a channel header button that let users create a Google Meet link from within Mattermost. Meetings are created using the Google account that each user connects through OAuth.

## Features

- Start a Google Meet meeting with `/meet start [topic]` or the channel header button.
- Create meetings as the currently connected Google user.
- Post a rich Mattermost message with a join link back into the channel.
- Optionally block meeting creation in public channels.
- Prompt users to connect or reconnect their Google account when OAuth is missing or stale.
- Hide the meeting button for non-admin users until the plugin is fully configured.

## User Experience

After the plugin is configured by a Mattermost administrator:

1. A user runs `/meet start` or clicks the Google Meet channel header button.
2. If the user has not connected Google yet, the plugin returns a connect link.
3. Once connected, the plugin creates a Meet link and posts it into the current channel.
4. `/meet start` optionally accepts a topic, for example:

```text
/meet start Sprint planning
```

The created post includes the meeting URL and an obvious join action in the Mattermost UI.

For backward compatibility, `/meet [topic]` still starts a meeting with an optional topic, and `/meet help` shows the available commands.

## Admin Setup

The plugin depends on Mattermost having a public `SiteURL` and on Google OAuth credentials for the Google Meet REST API.

### 1. Install and enable the plugin

Build the bundle locally:

```bash
make dist
```

This produces a plugin archive under `dist/`, typically:

```text
dist/com.mattermost.google-meet-<version>.tar.gz
```

Upload that archive in the Mattermost System Console and enable the plugin.

### 2. Ensure Mattermost Site URL is configured

Set `SiteURL` in Mattermost before configuring OAuth. The plugin uses it to build:

- the Google OAuth callback URL
- the Google connect URL
- the System Console configuration link shown to admins

If `SiteURL` is missing, the plugin intentionally reports itself as not fully configured.

### 3. Configure Google Cloud

In Google Cloud:

1. Enable the [Google Meet REST API](https://console.cloud.google.com/apis/library/meet.googleapis.com).
2. Create an OAuth 2.0 Client ID of type `Web application`.
3. Add the redirect URI shown in the plugin configuration page once the plugin is active.

The plugin displays the expected redirect URI in the System Console plugin settings header after activation.

### 4. Fill in plugin settings

In the Mattermost System Console, configure:

- `Google OAuth Client ID`: the Google OAuth client ID.
- `Google OAuth Client Secret`: the Google OAuth client secret.
- `Encryption Key`: the generated secret used to encrypt OAuth tokens at rest in Mattermost's KV store.
- `Restrict Meeting Creation`: when enabled, users can only create meetings in private channels, group messages, and direct messages.

Important notes:

- Rotating the `Encryption Key` invalidates existing stored Google connections.
- If Google scopes change or a token becomes unusable, the plugin asks the user to reconnect.

## Configuration Reference

### `GoogleClientID`

OAuth client ID used when sending users to Google for authorization.

### `GoogleClientSecret`

OAuth client secret used during token exchange and refresh. This field is marked secret in the plugin manifest and is masked in the System Console.

### `EncryptionKey`

Generated secret used to encrypt OAuth tokens before storing them in Mattermost. The plugin will not enable OAuth token operations until this is configured.

### `RestrictMeetingCreation`

When enabled, meeting creation is blocked in public channels. Users can still start meetings in private channels and direct-message surfaces.

## Development

### Requirements

- Go `1.25`
- Node.js `24.13.1` from `.nvmrc`
- npm compatible with that Node version

Use the repo's Node version with:

```bash
nvm use
```

Install webapp dependencies:

```bash
cd webapp && npm install
```

### Common commands

- `make test`: run server and webapp unit tests
- `make check-style`: run eslint, type checking, `go vet`, and `golangci-lint`
- `make dist`: build and bundle the plugin
- `make deploy`: build and deploy the plugin to a Mattermost server
- `make watch`: watch and rebuild webapp assets for development

### Local deployment

If Mattermost local mode is enabled, `make deploy` can deploy directly to your local server.

You can also deploy with API credentials or a personal access token, for example:

```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_TOKEN=YOUR_TOKEN
make deploy
```

For watch mode:

```bash
export MM_SERVICESETTINGS_SITEURL=http://localhost:8065
export MM_ADMIN_TOKEN=YOUR_TOKEN
make watch
```

If `MM_SERVICESETTINGS_ENABLEDEVELOPER` is set, the server build is limited to the current platform architecture to speed up local iteration.

## Testing

Run the full validation suite:

```bash
make test
make check-style
```

Server tests cover OAuth flow, token storage behavior, and meeting creation paths. Webapp tests cover the bundle manifest and basic React rendering.

## Release

The plugin version is derived at build time from git tags unless you explicitly maintain the manifest version yourself.

Release helpers:

- `make patch`
- `make minor`
- `make major`
- `make patch-rc`
- `make minor-rc`
- `make major-rc`

These targets create and push signed tags, so make sure you are on the correct branch and up to date before using them.

## Troubleshooting

### The plugin says it is not configured

Check all of the following:

- Mattermost `SiteURL` is set
- Google Meet REST API is enabled
- OAuth redirect URI in Google Cloud matches the one shown by the plugin
- `GoogleClientID`, `GoogleClientSecret`, and `EncryptionKey` are populated

### Users are asked to reconnect Google

This usually means one of:

- the token no longer has the required Google Meet scope
- the encryption key changed
- the stored token became unreadable or expired

Reconnecting refreshes the user's stored OAuth state.

### Users cannot start meetings in a channel

The plugin only posts meeting links into channels where the user has permission to create posts.

If `RestrictMeetingCreation` is enabled, public channels are also blocked even when the user has permission to post there.
