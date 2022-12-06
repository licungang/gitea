// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package incoming

import (
	"strings"
	"testing"

	"github.com/emersion/go-message/mail"
	"github.com/stretchr/testify/assert"
)

func TestIsAutomaticReply(t *testing.T) {
	cases := []struct {
		Headers  map[string][]string
		Expected bool
	}{
		{
			Headers:  map[string][]string{},
			Expected: false,
		},
		{
			Headers: map[string][]string{
				"Auto-Submitted": {"no"},
			},
			Expected: false,
		},
		{
			Headers: map[string][]string{
				"Auto-Submitted": {"yes"},
			},
			Expected: true,
		},
		{
			Headers: map[string][]string{
				"X-Autoreply": {"no"},
			},
			Expected: false,
		},
		{
			Headers: map[string][]string{
				"X-Autoreply": {"yes"},
			},
			Expected: true,
		},
		{
			Headers: map[string][]string{
				"X-Autorespond": {"yes"},
			},
			Expected: true,
		},
	}

	for _, c := range cases {
		assert.Equal(t, c.Expected, isAutomaticReply(mail.HeaderFromMap(c.Headers)))
	}
}

func TestGetContentFromMailReader(t *testing.T) {
	mailString := "Content-Type: multipart/mixed; boundary=message-boundary\r\n" +
		"\r\n" +
		"--message-boundary\r\n" +
		"Content-Type: multipart/alternative; boundary=text-boundary\r\n" +
		"\r\n" +
		"--text-boundary\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Disposition: inline\r\n" +
		"\r\n" +
		"mail content\r\n" +
		"--text-boundary--\r\n" +
		"--message-boundary\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Disposition: attachment; filename=attachment.txt\r\n" +
		"\r\n" +
		"attachment content\r\n" +
		"--message-boundary--\r\n"

	mr, err := mail.CreateReader(strings.NewReader(mailString))
	assert.NoError(t, err)
	content, err := getContentFromMailReader(mr)
	assert.NoError(t, err)
	assert.Equal(t, "mail content", content.Content)
	assert.Len(t, content.Attachments, 1)
	assert.Equal(t, "attachment.txt", content.Attachments[0].Name)
	assert.Equal(t, []byte("attachment content"), content.Attachments[0].Content.Bytes())

	mailString = "Content-Type: multipart/mixed; boundary=message-boundary\r\n" +
		"\r\n" +
		"--message-boundary\r\n" +
		"Content-Type: multipart/alternative; boundary=text-boundary\r\n" +
		"\r\n" +
		"--text-boundary\r\n" +
		"Content-Type: text/html\r\n" +
		"Content-Disposition: inline\r\n" +
		"\r\n" +
		"<p>mail content</p>\r\n" +
		"--text-boundary--\r\n" +
		"--message-boundary--\r\n"

	mr, err = mail.CreateReader(strings.NewReader(mailString))
	assert.NoError(t, err)
	content, err = getContentFromMailReader(mr)
	assert.NoError(t, err)
	assert.Equal(t, "mail content", content.Content)
	assert.Empty(t, content.Attachments)

	mailString = "Content-Type: multipart/mixed; boundary=message-boundary\r\n" +
		"\r\n" +
		"--message-boundary\r\n" +
		"Content-Type: multipart/alternative; boundary=text-boundary\r\n" +
		"\r\n" +
		"--text-boundary\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Disposition: inline\r\n" +
		"\r\n" +
		"mail content without signature\r\n" +
		"--\r\n" +
		"signature\r\n" +
		"--text-boundary--\r\n" +
		"--message-boundary--\r\n"

	mr, err = mail.CreateReader(strings.NewReader(mailString))
	assert.NoError(t, err)
	content, err = getContentFromMailReader(mr)
	assert.NoError(t, err)
	assert.Equal(t, "mail content without signature", content.Content)
	assert.Empty(t, content.Attachments)
}
