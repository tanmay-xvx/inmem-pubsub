#!/usr/bin/env python3
"""
Simple WebSocket test to verify basic connectivity
"""

import json
import websocket
import uuid

def on_message(ws, message):
    print(f"📨 Received: {message}")

def on_error(ws, error):
    print(f"❌ Error: {error}")

def on_close(ws, close_status_code, close_msg):
    print(f"🔌 Connection closed: {close_status_code} - {close_msg}")

def on_open(ws):
    print("✅ WebSocket connection opened!")
    
    # Test 1: Subscribe to orders topic
    subscribe_msg = {
        "type": "subscribe",
        "topic": "orders",
        "client_id": "test-client",
        "request_id": str(uuid.uuid4())
    }
    
    print(f"📤 Sending subscribe: {json.dumps(subscribe_msg, indent=2)}")
    ws.send(json.dumps(subscribe_msg))

def main():
    print("🚀 Starting simple WebSocket test...")
    
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
