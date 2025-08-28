#!/usr/bin/env python3
"""
WebSocket test client for inmem-pubsub system
Tests all required functionality according to assignment specification
"""

import json
import time
import uuid
import websocket
from datetime import datetime

class PubSubTester:
    def __init__(self, url="ws://localhost:8080/ws"):
        self.url = url
        self.ws = None
        self.client_id = f"test-client-{uuid.uuid4().hex[:8]}"
        
    def connect(self):
        """Establish WebSocket connection"""
        print(f"Connecting to {self.url}...")
        self.ws = websocket.create_connection(self.url)
        print("✅ Connected successfully!")
        
        # Wait for welcome message
        response = self.ws.recv()
        print(f"📨 Welcome message: {response}")
        
    def send_message(self, message):
        """Send a message and wait for response"""
        print(f"📤 Sending: {json.dumps(message, indent=2)}")
        self.ws.send(json.dumps(message))
        
        try:
            response = self.ws.recv()
            print(f"📨 Response: {response}")
            return json.loads(response)
        except Exception as e:
            print(f"❌ Error receiving response: {e}")
            return None
            
    def test_subscribe(self, topic, last_n=0):
        """Test subscribe functionality"""
        print(f"\n🔔 Testing SUBSCRIBE to topic '{topic}'")
        
        message = {
            "type": "subscribe",
            "topic": topic,
            "client_id": self.client_id,
            "last_n": last_n,
            "request_id": str(uuid.uuid4())
        }
        
        return self.send_message(message)
        
    def test_unsubscribe(self, topic):
        """Test unsubscribe functionality"""
        print(f"\n🔕 Testing UNSUBSCRIBE from topic '{topic}'")
        
        message = {
            "type": "unsubscribe",
            "topic": topic,
            "client_id": self.client_id,
            "request_id": str(uuid.uuid4())
        }
        
        return self.send_message(message)
        
    def test_publish(self, topic, payload):
        """Test publish functionality"""
        print(f"\n📢 Testing PUBLISH to topic '{topic}'")
        
        message = {
            "type": "publish",
            "topic": topic,
            "message": {
                "id": str(uuid.uuid4()),
                "payload": payload,
                "timestamp": datetime.now().isoformat()
            },
            "request_id": str(uuid.uuid4())
        }
        
        return self.send_message(message)
        
    def test_ping(self):
        """Test ping functionality"""
        print(f"\n🏓 Testing PING")
        
        message = {
            "type": "ping",
            "request_id": str(uuid.uuid4())
        }
        
        return self.send_message(message)
        
    def test_invalid_message(self):
        """Test error handling for invalid messages"""
        print(f"\n❌ Testing invalid message handling")
        
        message = {
            "type": "invalid_type",
            "request_id": str(uuid.uuid4())
        }
        
        return self.send_message(message)
        
    def close(self):
        """Close the WebSocket connection"""
        if self.ws:
            self.ws.close()
            print("🔌 Connection closed")

def main():
    print("🚀 Starting WebSocket functionality tests...")
    print("=" * 50)
    
    tester = PubSubTester()
    
    try:
        # Connect to WebSocket
        tester.connect()
        
        # Test 1: Subscribe to orders topic
        response = tester.test_subscribe("orders", last_n=5)
        if response and response.get("type") == "ack":
            print("✅ Subscribe test PASSED")
        else:
            print("❌ Subscribe test FAILED")
            
        # Test 2: Subscribe to news topic
        response = tester.test_subscribe("news")
        if response and response.get("type") == "ack":
            print("✅ Subscribe to news topic PASSED")
        else:
            print("❌ Subscribe to news topic FAILED")
            
        # Test 3: Publish message to orders topic
        payload = {
            "order_id": "ORD-123",
            "amount": 99.50,
            "currency": "USD"
        }
        response = tester.test_publish("orders", payload)
        if response and response.get("type") == "ack":
            print("✅ Publish test PASSED")
        else:
            print("❌ Publish test FAILED")
            
        # Test 4: Publish message to news topic
        news_payload = {
            "headline": "Breaking News!",
            "content": "This is a test news article",
            "priority": "high"
        }
        response = tester.test_publish("news", news_payload)
        if response and response.get("type") == "ack":
            print("✅ Publish to news topic PASSED")
        else:
            print("❌ Publish to news topic FAILED")
            
        # Test 5: Ping test
        response = tester.test_ping()
        if response and response.get("type") == "pong":
            print("✅ Ping test PASSED")
        else:
            print("❌ Ping test FAILED")
            
        # Test 6: Test invalid message type
        response = tester.test_invalid_message()
        if response and response.get("type") == "error":
            print("✅ Error handling test PASSED")
        else:
            print("❌ Error handling test FAILED")
            
        # Test 7: Unsubscribe from orders topic
        response = tester.test_unsubscribe("orders")
        if response and response.get("type") == "ack":
            print("✅ Unsubscribe test PASSED")
        else:
            print("❌ Unsubscribe test FAILED")
            
        # Test 8: Try to publish to non-existent topic
        print(f"\n❌ Testing PUBLISH to non-existent topic")
        response = tester.test_publish("nonexistent", {"test": "data"})
        if response and response.get("type") == "error":
            print("✅ Non-existent topic error handling PASSED")
        else:
            print("❌ Non-existent topic error handling FAILED")
            
        print("\n" + "=" * 50)
        print("🎉 All WebSocket tests completed!")
        
    except Exception as e:
        print(f"❌ Test failed with error: {e}")
        
    finally:
        tester.close()

if __name__ == "__main__":
    main()
