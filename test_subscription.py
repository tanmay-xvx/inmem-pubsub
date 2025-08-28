#!/usr/bin/env python3
"""
Test WebSocket subscription functionality
"""

import json
import websocket
import uuid
import time

responses_received = []

def on_message(ws, message):
    print(f"📨 Received: {message}")
    try:
        data = json.loads(message)
        responses_received.append(data)
        
        if data.get("type") == "ack":
            print("✅ Received acknowledgment!")
        elif data.get("type") == "event":
            print("📢 Received published message!")
        elif data.get("type") == "connected":
            print("🔌 Received connection confirmation!")
        elif data.get("type") == "error":
            print("❌ Received error message!")
        elif data.get("type") == "pong":
            print("🏓 Received pong!")
    except json.JSONDecodeError:
        print("❌ Invalid JSON received")

def on_error(ws, error):
    print(f"❌ WebSocket error: {error}")

def on_close(ws, close_status_code, close_msg):
    print(f"🔌 Connection closed: {close_status_code} - {close_msg}")

def on_open(ws):
    print("✅ WebSocket connection opened!")
    
    # Wait a moment for connection to stabilize
    time.sleep(0.5)
    
    # Test 1: Subscribe to orders topic
    subscribe_msg = {
        "type": "subscribe",
        "topic": "orders",
        "client_id": "test-client",
        "request_id": str(uuid.uuid4())
    }
    
    print(f"📤 Sending subscribe: {json.dumps(subscribe_msg, indent=2)}")
    ws.send(json.dumps(subscribe_msg))
    
    # Wait for ack
    time.sleep(2)
    
    # Test 2: Publish a message
    publish_msg = {
        "type": "publish",
        "topic": "orders",
        "message": {
            "id": str(uuid.uuid4()),
            "payload": {"test": "Hello World!"},
            "timestamp": "2024-01-01T12:00:00Z"
        },
        "request_id": str(uuid.uuid4())
    }
    
    print(f"📤 Sending publish: {json.dumps(publish_msg, indent=2)}")
    ws.send(json.dumps(publish_msg))
    
    # Wait for ack and message
    time.sleep(3)
    
    # Test 3: Unsubscribe
    unsubscribe_msg = {
        "type": "unsubscribe",
        "topic": "orders",
        "client_id": "test-client",
        "request_id": str(uuid.uuid4())
    }
    
    print(f"📤 Sending unsubscribe: {json.dumps(unsubscribe_msg, indent=2)}")
    ws.send(json.dumps(unsubscribe_msg))
    
    # Wait for ack
    time.sleep(2)
    
    print("🎉 Test completed!")
    print(f"📊 Total responses received: {len(responses_received)}")
    for i, response in enumerate(responses_received):
        print(f"   {i+1}. {response.get('type', 'unknown')}: {response}")
    
    ws.close()

def main():
    print("🚀 Starting WebSocket subscription test...")
    
    # Create WebSocket connection
    ws = websocket.WebSocketApp(
        "ws://localhost:8080/ws",
        on_open=on_open,
        on_message=on_message,
        on_error=on_error,
        on_close=on_close
    )
    
    # Run the WebSocket connection
    ws.run_forever()

if __name__ == "__main__":
    main()
