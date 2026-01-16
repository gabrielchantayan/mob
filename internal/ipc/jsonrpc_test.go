package ipc

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestRequest_Marshal(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test/method",
		Params: map[string]string{
			"key": "value",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Unmarshal to verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if parsed["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %v", parsed["jsonrpc"])
	}
	if parsed["id"].(float64) != 1 {
		t.Errorf("expected id=1, got %v", parsed["id"])
	}
	if parsed["method"] != "test/method" {
		t.Errorf("expected method=test/method, got %v", parsed["method"])
	}
	params := parsed["params"].(map[string]interface{})
	if params["key"] != "value" {
		t.Errorf("expected params.key=value, got %v", params["key"])
	}
}

func TestRequest_MarshalNoParams(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test/method",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Verify params field is omitted
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if _, exists := parsed["params"]; exists {
		t.Errorf("expected params to be omitted when nil")
	}
}

func TestResponse_Unmarshal(t *testing.T) {
	jsonData := `{"jsonrpc":"2.0","id":1,"result":{"status":"ok"}}`

	var resp Response
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %s", resp.JSONRPC)
	}
	if resp.ID != 1 {
		t.Errorf("expected id=1, got %d", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("expected no error, got %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}

	// Parse the result
	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", result["status"])
	}
}

func TestResponse_UnmarshalError(t *testing.T) {
	jsonData := `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid Request","data":"extra info"}}`

	var resp Response
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %s", resp.JSONRPC)
	}
	if resp.ID != 1 {
		t.Errorf("expected id=1, got %d", resp.ID)
	}
	if resp.Result != nil {
		t.Errorf("expected no result, got %v", resp.Result)
	}
	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if resp.Error.Code != -32600 {
		t.Errorf("expected error code=-32600, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "Invalid Request" {
		t.Errorf("expected error message='Invalid Request', got %s", resp.Error.Message)
	}
	if resp.Error.Data != "extra info" {
		t.Errorf("expected error data='extra info', got %v", resp.Error.Data)
	}
}

func TestRPCError_Error(t *testing.T) {
	err := &RPCError{
		Code:    -32600,
		Message: "Invalid Request",
	}

	errStr := err.Error()
	if errStr != "rpc error: code=-32600, message=Invalid Request" {
		t.Errorf("unexpected error string: %s", errStr)
	}
}

func TestClient_Call(t *testing.T) {
	// Create a pipe to simulate stdin/stdout
	clientToServer := &bytes.Buffer{}
	serverToClient := &bytes.Buffer{}

	// Pre-write a response that the client will read
	resp := Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  json.RawMessage(`{"status":"success"}`),
	}
	if err := json.NewEncoder(serverToClient).Encode(resp); err != nil {
		t.Fatalf("failed to encode response: %v", err)
	}

	client := NewClient(clientToServer, serverToClient)

	response, err := client.Call("test/method", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	// Verify the request was sent correctly
	var sentReq Request
	if err := json.NewDecoder(clientToServer).Decode(&sentReq); err != nil {
		t.Fatalf("failed to decode sent request: %v", err)
	}
	if sentReq.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %s", sentReq.JSONRPC)
	}
	if sentReq.ID != 1 {
		t.Errorf("expected id=1, got %d", sentReq.ID)
	}
	if sentReq.Method != "test/method" {
		t.Errorf("expected method=test/method, got %s", sentReq.Method)
	}

	// Verify the response
	if response.ID != 1 {
		t.Errorf("expected response id=1, got %d", response.ID)
	}
	if response.Error != nil {
		t.Errorf("expected no error, got %v", response.Error)
	}
}

func TestClient_Send(t *testing.T) {
	// Create a buffer to capture what the client sends
	clientToServer := &bytes.Buffer{}

	client := NewClient(clientToServer, &bytes.Buffer{})

	err := client.Send("notifications/message", map[string]string{"text": "hello"})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify the notification was sent correctly
	var sentReq Request
	if err := json.NewDecoder(clientToServer).Decode(&sentReq); err != nil {
		t.Fatalf("failed to decode sent request: %v", err)
	}
	if sentReq.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %s", sentReq.JSONRPC)
	}
	// Notifications should still have an ID in JSON-RPC 2.0 if we want to track them
	// But true notifications have no ID - let's check the spec behavior
	if sentReq.Method != "notifications/message" {
		t.Errorf("expected method=notifications/message, got %s", sentReq.Method)
	}
}

func TestClient_Receive(t *testing.T) {
	serverToClient := &bytes.Buffer{}

	// Write a response
	resp := Response{
		JSONRPC: "2.0",
		ID:      42,
		Result:  json.RawMessage(`{"data":"test"}`),
	}
	if err := json.NewEncoder(serverToClient).Encode(resp); err != nil {
		t.Fatalf("failed to encode response: %v", err)
	}

	client := NewClient(&bytes.Buffer{}, serverToClient)

	received, err := client.Receive()
	if err != nil {
		t.Fatalf("Receive failed: %v", err)
	}

	if received.ID != 42 {
		t.Errorf("expected id=42, got %d", received.ID)
	}
	if received.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %s", received.JSONRPC)
	}
}

func TestClient_ReceiveEOF(t *testing.T) {
	// Empty buffer simulates EOF
	serverToClient := &bytes.Buffer{}

	client := NewClient(&bytes.Buffer{}, serverToClient)

	_, err := client.Receive()
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
}

func TestClient_CallIncrementsID(t *testing.T) {
	clientToServer := &bytes.Buffer{}
	serverToClient := &bytes.Buffer{}

	// Pre-write two responses
	for i := 1; i <= 2; i++ {
		resp := Response{
			JSONRPC: "2.0",
			ID:      i,
			Result:  json.RawMessage(`{}`),
		}
		if err := json.NewEncoder(serverToClient).Encode(resp); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}

	client := NewClient(clientToServer, serverToClient)

	// First call
	_, err := client.Call("method1", nil)
	if err != nil {
		t.Fatalf("first Call failed: %v", err)
	}

	// Second call
	_, err = client.Call("method2", nil)
	if err != nil {
		t.Fatalf("second Call failed: %v", err)
	}

	// Verify IDs incremented
	dec := json.NewDecoder(clientToServer)

	var req1 Request
	if err := dec.Decode(&req1); err != nil {
		t.Fatalf("failed to decode first request: %v", err)
	}
	if req1.ID != 1 {
		t.Errorf("expected first request id=1, got %d", req1.ID)
	}

	var req2 Request
	if err := dec.Decode(&req2); err != nil {
		t.Fatalf("failed to decode second request: %v", err)
	}
	if req2.ID != 2 {
		t.Errorf("expected second request id=2, got %d", req2.ID)
	}
}

func TestClient_Close(t *testing.T) {
	client := NewClient(&bytes.Buffer{}, &bytes.Buffer{})

	err := client.Close()
	if err != nil {
		t.Errorf("Close should return nil, got %v", err)
	}
}
