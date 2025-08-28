#!/usr/bin/env python3
"""
Proper WebSocket test that waits for responses
"""

import json
import websocket
import uuid
import time
import threading
import queue

class WebSocketTester:
    def __init__(self):
        self.responses = queue.Queue()
        self.ws = None
        self.connected = False
        
    def on_message(self, ws, message):
        print(f"📨 Received: {message}")
        try:
            data = json.loads(message)
            self.responses.put(data)
            
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

    def on_error(self, ws, error):
        print(f"❌ WebSocket error: {error}")

    def on_close(self, ws, close_status_code, close_msg):
        print(f"🔌 Connection closed: {close_status_code} - {close_msg}")
        self.connected = False

    def on_open(self, ws):
        print("✅ WebSocket connection opened!")
        self.connected = True

    def wait_for_response(self, timeout=5):
        """Wait for a response message"""
        try:
            response = self.responses.get(timeout=timeout)
            return response
        except queue.Empty:
            return None

    def send_and_wait(self, message, wait_time=2):
        """Send a message and wait for a response"""
        print(f"📤 Sending: {json.dumps(message, indent=2)}")
        self.ws.send(json.dumps(message))
        
        # Wait for response
        time.sleep(wait_time)
        
        # Collect any responses that came in
        responses = []
        try:
            while True:
                response = self.responses.get_nowait()
                responses.append(response)
        except queue.Empty:
            pass
            
        return responses

    def test_websocket(self):
        print("🚀 Starting comprehensive WebSocket test...")
        
        # Create WebSocket connection
        self.ws = websocket.WebSocketApp(
            "ws://localhost:8080/ws",
            on_open=self.on_open,
            on_message=self.on_message,
            on_error=self.on_error,
            on_close=self.on_close
        )
        
        # Start WebSocket in a separate thread
        def run_ws():
            self.ws.run_forever()
            
        ws_thread = threading.Thread(target=run_ws)
        ws_thread.daemon = True
        ws_thread.start()
        
        # Wait for connection
        time.sleep(1)
        if not self.connected:
            print("❌ Failed to connect to WebSocket")
            return
            
        try:
            # Test 1: Subscribe to orders topic
            subscribe_msg = {
                "type": "subscribe",
                "topic": "orders",
                "client_id": "test-client",
                "request_id": str(uuid.uuid4())
            }
            
            responses = self.send_and_wait(subscribe_msg, 3)
            print(f"📊 Subscribe responses: {len(responses)}")
            for resp in responses:
                print(f"   - {resp}")
            
            # Test 2: Publish a message
            publish_msg = {
                "type": "publish",
                "topic": "orders",
                "message": {
                    "id": str(uuid.uuid4()),
                    "payload": {"test": "Hello World!", "timestamp": "2024-01-01T12:00:00Z"},
                },
                "request_id": str(uuid.uuid4())
            }
            
            responses = self.send_and_wait(publish_msg, 3)
            print(f"📊 Publish responses: {len(responses)}")
            for resp in responses:
                print(f"   - {resp}")
            
            # Test 3: Ping
            ping_msg = {
                "type": "ping",
                "request_id": str(uuid.uuid4())
            }
            
            responses = self.send_and_wait(ping_msg, 3)
            print(f"📊 Ping responses: {len(responses)}")
            for resp in responses:
                print(f"   - {resp}")
            
            # Test 4: Unsubscribe
            unsubscribe_msg = {
                "type": "unsubscribe",
                "topic": "orders",
                "client_id": "test-client",
                "request_id": str(uuid.uuid4())
            }
            
            responses = self.send_and_wait(unsubscribe_msg, 3)
            print(f"📊 Unsubscribe responses: {len(responses)}")
            for resp in responses:
                print(f"   - {resp}")
                
        finally:
            print("🎉 Test completed!")
            self.ws.close()
            
            # Wait a bit for cleanup
            time.sleep(1)

def main():
    tester = WebSocketTester()
    tester.test_websocket()

if __name__ == "__main__":
    main()
