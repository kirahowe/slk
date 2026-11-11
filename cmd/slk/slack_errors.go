package main

import (
	"errors"

	"github.com/slack-go/slack"
)

// isMessageNotFound reports whether err is Slack's message_not_found:
// the message a chat.update or chat.delete named does not exist (it
// was deleted).
func isMessageNotFound(err error) bool {
	var refused slack.SlackErrorResponse
	return errors.As(err, &refused) && refused.Err == "message_not_found"
}

// isNoSuchUser reports whether err is users.info saying the ID names no
// user this token can see: user_not_found (deleted, never existed, not
// a user ID) or user_not_visible (a user of an organisation this one
// shares nothing with). Asking again in the same session gets the same
// answer.
func isNoSuchUser(err error) bool {
	var refused slack.SlackErrorResponse
	return errors.As(err, &refused) && (refused.Err == "user_not_found" || refused.Err == "user_not_visible")
}

// isNoSuchBot reports whether err is bots.info's bot_not_found: the ID
// names no bot (Slack's documented answer for an unknown bot). Asking
// again in the same session gets the same answer.
func isNoSuchBot(err error) bool {
	var refused slack.SlackErrorResponse
	return errors.As(err, &refused) && refused.Err == "bot_not_found"
}
