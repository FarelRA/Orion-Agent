// Package send provides a comprehensive abstraction layer for sending messages via WhatsApp.
//
// This package covers all whatsmeow sending capabilities including:
//   - Text messages (simple and extended with link previews, mentions)
//   - Media messages (image, video, audio, document, sticker)
//   - Location messages (static and live)
//   - Contact messages (single and array)
//   - Interactive messages (polls, events, group invites)
//   - Message operations (edit, revoke, react, reply, forward, pin, star)
//   - Presence management (typing, recording, online/offline, mark read)
//
// # Basic Usage
//
//	sender := send.NewSendService(waClient, jidService, log)
//
//	// Send text
//	sender.Send(ctx, recipientJID, send.Text("Hello!"))
//
//	// Send image with caption
//	sender.Send(ctx, recipientJID, send.ImageWithCaption(data, "image/jpeg", "Caption"))
//
//	// Send voice note
//	sender.Send(ctx, recipientJID, send.VoiceNote(audioData, 30))
//
//	// Send poll
//	sender.Send(ctx, groupJID, send.Poll("Question?", []string{"Option A", "Option B"}))
//
// # Replies
//
//	sender.Reply(ctx, chatJID, originalMsgID, senderJID, send.Text("Reply text"))
//
// # Reactions
//
//	sender.React(ctx, chatJID, msgID, senderJID, "👍")
//	sender.RemoveReaction(ctx, chatJID, msgID, senderJID)
//
// # Message Operations
//
//	sender.Edit(ctx, chatJID, msgID, "New text")
//	sender.RevokeOwn(ctx, chatJID, msgID)
//	sender.Forward(ctx, toJID, originalMessage)
//
// # Presence
//
//	sender.StartTyping(ctx, chatJID)
//	sender.StopTyping(ctx, chatJID)
//	sender.MarkRead(ctx, chatJID, senderJID, msgID1, msgID2)
package send
