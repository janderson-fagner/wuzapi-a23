package main

import (
	"testing"
)

func TestChatListRouting(t *testing.T) {
	s := makeTestServer(t)

	// Create a user first
	addRequest := newRequest("1", "admin.users.add", map[string]interface{}{
		"adminToken": "test-admin-token",
		"name":       "ListUser",
		"token":      "list-token",
	}).toJSON(t)
	executeRequest(t, s, addRequest)

	// Try to list chats (will fail because no WhatsApp session, but tests routing)
	listRequest := newRequest("2", "chat.list", map[string]interface{}{
		"token": "list-token",
	}).toJSON(t)
	listResponse := executeRequest(t, s, listRequest)

	expected := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "2",
	}
	if diff := compareJSON(expected, listResponse); diff != "" {
		t.Errorf("Response mismatch:\n%s", diff)
	}

	// Either result or error should be present
	if listResponse["result"] == nil && listResponse["error"] == nil {
		t.Errorf("Expected either result or error field")
	}
}

func TestChatMarkUnreadRouting(t *testing.T) {
	s := makeTestServer(t)

	addRequest := newRequest("1", "admin.users.add", map[string]interface{}{
		"adminToken": "test-admin-token",
		"name":       "UnreadUser",
		"token":      "unread-token",
	}).toJSON(t)
	executeRequest(t, s, addRequest)

	// Try to mark unread (no session, tests routing)
	unreadRequest := newRequest("2", "chat.markunread", map[string]interface{}{
		"token": "unread-token",
		"jid":   "5511999@s.whatsapp.net",
	}).toJSON(t)
	unreadResponse := executeRequest(t, s, unreadRequest)

	assertJSONRPC20Error(t, unreadResponse, "2", 500) // "no session"
}

func TestChatMarkUnreadMissingJid(t *testing.T) {
	s := makeTestServer(t)

	addRequest := newRequest("1", "admin.users.add", map[string]interface{}{
		"adminToken": "test-admin-token",
		"name":       "UnreadUser2",
		"token":      "unread-token-2",
	}).toJSON(t)
	executeRequest(t, s, addRequest)

	// Missing jid should return 400 - but no session so 500 first
	unreadRequest := newRequest("2", "chat.markunread", map[string]interface{}{
		"token": "unread-token-2",
	}).toJSON(t)
	unreadResponse := executeRequest(t, s, unreadRequest)

	// Should get error (either no session 500 or missing jid 400)
	if unreadResponse["error"] == nil {
		t.Errorf("Expected error response")
	}
}

func TestChatPinRouting(t *testing.T) {
	s := makeTestServer(t)

	addRequest := newRequest("1", "admin.users.add", map[string]interface{}{
		"adminToken": "test-admin-token",
		"name":       "PinUser",
		"token":      "pin-token",
	}).toJSON(t)
	executeRequest(t, s, addRequest)

	pinRequest := newRequest("2", "chat.pin", map[string]interface{}{
		"token": "pin-token",
		"jid":   "5511999@s.whatsapp.net",
		"pin":   true,
	}).toJSON(t)
	pinResponse := executeRequest(t, s, pinRequest)

	assertJSONRPC20Error(t, pinResponse, "2", 500) // "no session"
}

func TestLabelEditRouting(t *testing.T) {
	s := makeTestServer(t)

	addRequest := newRequest("1", "admin.users.add", map[string]interface{}{
		"adminToken": "test-admin-token",
		"name":       "LabelUser",
		"token":      "label-token",
	}).toJSON(t)
	executeRequest(t, s, addRequest)

	labelRequest := newRequest("2", "label.edit", map[string]interface{}{
		"token":       "label-token",
		"label_id":    "1",
		"label_name":  "Important",
		"label_color": 0,
		"deleted":     false,
	}).toJSON(t)
	labelResponse := executeRequest(t, s, labelRequest)

	assertJSONRPC20Error(t, labelResponse, "2", 500) // "no session"
}

func TestLabelEditMissingLabelId(t *testing.T) {
	s := makeTestServer(t)

	addRequest := newRequest("1", "admin.users.add", map[string]interface{}{
		"adminToken": "test-admin-token",
		"name":       "LabelUser2",
		"token":      "label-token-2",
	}).toJSON(t)
	executeRequest(t, s, addRequest)

	// Missing label_id
	labelRequest := newRequest("2", "label.edit", map[string]interface{}{
		"token":      "label-token-2",
		"label_name": "Important",
	}).toJSON(t)
	labelResponse := executeRequest(t, s, labelRequest)

	// Should get error (no session 500 comes before validation)
	if labelResponse["error"] == nil {
		t.Errorf("Expected error response")
	}
}

func TestLabelChatRouting(t *testing.T) {
	s := makeTestServer(t)

	addRequest := newRequest("1", "admin.users.add", map[string]interface{}{
		"adminToken": "test-admin-token",
		"name":       "LabelChatUser",
		"token":      "labelchat-token",
	}).toJSON(t)
	executeRequest(t, s, addRequest)

	labelChatRequest := newRequest("2", "label.chat", map[string]interface{}{
		"token":    "labelchat-token",
		"jid":      "5511999@s.whatsapp.net",
		"label_id": "1",
		"labeled":  true,
	}).toJSON(t)
	labelChatResponse := executeRequest(t, s, labelChatRequest)

	assertJSONRPC20Error(t, labelChatResponse, "2", 500) // "no session"
}

func TestLabelMessageRouting(t *testing.T) {
	s := makeTestServer(t)

	addRequest := newRequest("1", "admin.users.add", map[string]interface{}{
		"adminToken": "test-admin-token",
		"name":       "LabelMsgUser",
		"token":      "labelmsg-token",
	}).toJSON(t)
	executeRequest(t, s, addRequest)

	labelMsgRequest := newRequest("2", "label.message", map[string]interface{}{
		"token":      "labelmsg-token",
		"jid":        "5511999@s.whatsapp.net",
		"label_id":   "1",
		"message_id": "ABC123",
		"labeled":    true,
	}).toJSON(t)
	labelMsgResponse := executeRequest(t, s, labelMsgRequest)

	assertJSONRPC20Error(t, labelMsgResponse, "2", 500) // "no session"
}

func TestUnknownMethodReturns404(t *testing.T) {
	s := makeTestServer(t)

	request := newRequest("1", "nonexistent.method", nil).toJSON(t)
	response := executeRequest(t, s, request)

	assertJSONRPC20Error(t, response, "1", 404)
}
