package talk

import (
	"reflect"
	discussion "writer/ext/talk/engine"
	"writer/ext/talk/talktext"
	"writer/locales"
)

func translate(err error, reason func(error) string) (string, bool) {
	switch cause := err.(type) {
	case *discussion.ReplyError:
		return talktext.ChatReplyError(reason(cause.Err)), true
	case *discussion.CompactError:
		return talktext.ChatCompactError(reason(cause.Err)), true

	}
	if err != nil && reflect.TypeOf(err).Comparable() {
		if label, ok := talkErrors[err]; ok {
			return label(), true
		}
	}
	return "", false
}

var talkErrors = map[error]func() string{
	discussion.ErrChapterGone:      locales.ChatChapterGone,
	discussion.ErrNoModelReason:    locales.ChatNeedModel,
	discussion.ErrInputRequired:    talktext.ChatInputRequired,
	discussion.ErrNewChatRequired:  talktext.ChatNewChatRequired,
	discussion.ErrAlreadyReplied:   talktext.ChatAlreadyReplied,
	discussion.ErrRetryMissing:     talktext.ChatRetryMissing,
	discussion.ErrEmptyReply:       talktext.ChatEmptyReply,
	discussion.ErrSelectionInvalid: talktext.ChatSelectionInvalid,
	discussion.ErrSelectionBlank:   talktext.ChatSelectionBlank,
	discussion.ErrSourceMissing:    talktext.ChatSourceMissing,
	discussion.ErrEmptyDigest:      talktext.ChatEmptyDigest,
}
