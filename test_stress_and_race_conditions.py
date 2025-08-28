#!/usr/bin/env python3
"""
Comprehensive stress test for inmem-pubsub service.
Tests race conditions, large data volumes, and ring buffer functionality.
"""

import asyncio
import json
import time
import uuid
import threading
import concurrent.futures
import websocket
import requests
import statistics
from typing import List, Dict, Any
from dataclasses import dataclass, field
from datetime import datetime
import signal
import sys

@dataclass
class TestResults:
    """Container for test results and metrics"""
    messages_sent: int = 0
    messages_received: int = 0
    errors: List[str] = field(default_factory=list)
    latencies: List[float] = field(default_factory=list)
    start_time: float = 0
    end_time: float = 0
    
    @property
    def duration(self) -> float:
        return self.end_time - self.start_time
    
    @property
    def throughput(self) -> float:
        if self.duration > 0:
            return self.messages_received / self.duration
        return 0
    
    @property
    def avg_latency(self) -> float:
        return statistics.mean(self.latencies) if self.latencies else 0
    
    @property
    def p95_latency(self) -> float:
        if self.latencies:
            sorted_latencies = sorted(self.latencies)
            idx = int(0.95 * len(sorted_latencies))
            return sorted_latencies[idx]
        return 0

class WebSocketClient:
    """Thread-safe WebSocket client for testing"""
    
    def __init__(self, url: str, client_id: str):
        self.url = url
        self.client_id = client_id
        self.ws = None
        self.messages_received = []
        self.responses_received = []
        self.connected = False
        self.lock = threading.Lock()
        self.message_times = {}  # Track message send times for latency
        
    def connect(self):
        """Connect to WebSocket server"""
        try:
            self.ws = websocket.WebSocketApp(
                self.url,
                on_open=self.on_open,
                on_message=self.on_message,
                on_error=self.on_error,
                on_close=self.on_close
            )
            
            # Start WebSocket in a separate thread
            self.ws_thread = threading.Thread(target=self.ws.run_forever)
            self.ws_thread.daemon = True
            self.ws_thread.start()
            
            # Wait for connection
            timeout = 10
            while not self.connected and timeout > 0:
                time.sleep(0.1)
                timeout -= 0.1
                
            return self.connected
        except Exception as e:
            print(f"Connection error for {self.client_id}: {e}")
            return False
    
    def on_open(self, ws):
        print(f"Client {self.client_id} connected")
        self.connected = True
    
    def on_message(self, ws, message):
        try:
            msg = json.loads(message)
            with self.lock:
                self.responses_received.append(msg)
                
                # Calculate latency for published messages
                if msg.get("type") == "event" and "message" in msg:
                    msg_id = msg["message"].get("id")
                    if msg_id in self.message_times:
                        latency = time.time() - self.message_times[msg_id]
                        self.messages_received.append((msg, latency))
                        del self.message_times[msg_id]
                elif msg.get("type") == "ack":
                    request_id = msg.get("request_id")
                    if request_id in self.message_times:
                        latency = time.time() - self.message_times[request_id]
                        del self.message_times[request_id]
                        
        except Exception as e:
            print(f"Message parsing error for {self.client_id}: {e}")
    
    def on_error(self, ws, error):
        print(f"WebSocket error for {self.client_id}: {error}")
    
    def on_close(self, ws, close_status_code, close_msg):
        print(f"Client {self.client_id} disconnected")
        self.connected = False
    
    def send_message(self, message: Dict[str, Any]) -> bool:
        """Send a message and track timing"""
        if not self.connected or not self.ws:
            return False
            
        try:
            request_id = str(uuid.uuid4())
            message["request_id"] = request_id
            
            with self.lock:
                self.message_times[request_id] = time.time()
                
            self.ws.send(json.dumps(message))
            return True
        except Exception as e:
            print(f"Send error for {self.client_id}: {e}")
            return False
    
    def subscribe(self, topic: str, last_n: int = 0) -> bool:
        """Subscribe to a topic"""
        message = {
            "type": "subscribe",
            "topic": topic,
            "client_id": self.client_id,
            "last_n": last_n
        }
        return self.send_message(message)
    
    def publish(self, topic: str, payload: Any) -> bool:
        """Publish a message to a topic"""
        message_id = str(uuid.uuid4())
        message = {
            "type": "publish",
            "topic": topic,
            "message": {
                "id": message_id,
                "payload": payload
            }
        }
        
        # Track message ID for latency calculation
        with self.lock:
            self.message_times[message_id] = time.time()
            
        return self.send_message(message)
    
    def unsubscribe(self, topic: str) -> bool:
        """Unsubscribe from a topic"""
        message = {
            "type": "unsubscribe",
            "topic": topic,
            "client_id": self.client_id
        }
        return self.send_message(message)
    
    def ping(self) -> bool:
        """Send a ping message"""
        message = {"type": "ping"}
        return self.send_message(message)
    
    def close(self):
        """Close the WebSocket connection"""
        if self.ws:
            self.ws.close()
        self.connected = False

class StressTestSuite:
    """Comprehensive stress test suite"""
    
    def __init__(self, base_url: str = "http://localhost:8080", ws_url: str = "ws://localhost:8080/ws"):
        self.base_url = base_url
        self.ws_url = ws_url
        self.clients = []
        self.results = TestResults()
        
    def setup_topics(self, topics: List[str]):
        """Create test topics via REST API"""
        for topic in topics:
            try:
                response = requests.post(f"{self.base_url}/topics", json={"name": topic})
                if response.status_code not in [201, 409]:  # Created or already exists
                    print(f"Failed to create topic {topic}: {response.status_code}")
            except Exception as e:
                print(f"Error creating topic {topic}: {e}")
    
    def cleanup_topics(self, topics: List[str]):
        """Delete test topics via REST API"""
        for topic in topics:
            try:
                response = requests.delete(f"{self.base_url}/topics/{topic}")
                if response.status_code not in [200, 404]:  # Deleted or not found
                    print(f"Failed to delete topic {topic}: {response.status_code}")
            except Exception as e:
                print(f"Error deleting topic {topic}: {e}")
    
    def test_race_conditions(self, num_publishers: int = 10, num_subscribers: int = 10, 
                           messages_per_publisher: int = 100, topic: str = "race-test"):
        """Test for race conditions with concurrent publishers and subscribers"""
        print(f"\n🏁 Testing race conditions: {num_publishers} publishers, {num_subscribers} subscribers")
        
        self.setup_topics([topic])
        
        # Create subscribers
        subscribers = []
        for i in range(num_subscribers):
            client = WebSocketClient(self.ws_url, f"subscriber-{i}")
            if client.connect():
                client.subscribe(topic)
                subscribers.append(client)
                time.sleep(0.01)  # Small delay to avoid overwhelming
        
        print(f"Connected {len(subscribers)} subscribers")
        
        # Create publishers
        publishers = []
        for i in range(num_publishers):
            client = WebSocketClient(self.ws_url, f"publisher-{i}")
            if client.connect():
                publishers.append(client)
                time.sleep(0.01)
        
        print(f"Connected {len(publishers)} publishers")
        
        # Concurrent publishing
        def publish_messages(publisher, publisher_id):
            messages_sent = 0
            for j in range(messages_per_publisher):
                payload = {
                    "publisher_id": publisher_id,
                    "message_num": j,
                    "timestamp": time.time(),
                    "data": f"Race test message {j} from publisher {publisher_id}"
                }
                
                if publisher.publish(topic, payload):
                    messages_sent += 1
                
                # Random small delay to create more realistic race conditions
                time.sleep(0.001 + (j % 10) * 0.0001)
            
            return messages_sent
        
        # Start concurrent publishing
        start_time = time.time()
        with concurrent.futures.ThreadPoolExecutor(max_workers=num_publishers) as executor:
            futures = [
                executor.submit(publish_messages, pub, i) 
                for i, pub in enumerate(publishers)
            ]
            
            total_sent = sum(future.result() for future in concurrent.futures.as_completed(futures))
        
        # Wait for message propagation
        time.sleep(2)
        
        end_time = time.time()
        duration = end_time - start_time
        
        # Collect results
        total_received = 0
        all_latencies = []
        
        for subscriber in subscribers:
            with subscriber.lock:
                received_count = len(subscriber.messages_received)
                total_received += received_count
                all_latencies.extend([latency for _, latency in subscriber.messages_received])
        
        # Calculate expected vs actual
        expected_total = total_sent * len(subscribers)  # Each subscriber should get all messages
        
        print(f"📊 Race Condition Test Results:")
        print(f"   Duration: {duration:.2f}s")
        print(f"   Messages sent: {total_sent}")
        print(f"   Expected total received: {expected_total}")
        print(f"   Actual total received: {total_received}")
        print(f"   Success rate: {(total_received/expected_total)*100:.1f}%" if expected_total > 0 else "N/A")
        print(f"   Avg latency: {statistics.mean(all_latencies):.3f}s" if all_latencies else "N/A")
        print(f"   P95 latency: {sorted(all_latencies)[int(0.95*len(all_latencies))]:.3f}s" if all_latencies else "N/A")
        
        # Cleanup
        for client in subscribers + publishers:
            client.close()
        
        self.cleanup_topics([topic])
        
        return {
            "messages_sent": total_sent,
            "messages_received": total_received,
            "expected_received": expected_total,
            "success_rate": (total_received/expected_total) if expected_total > 0 else 0,
            "duration": duration,
            "latencies": all_latencies
        }
    
    def test_large_data_volume(self, message_size_kb: int = 10, num_messages: int = 1000, 
                              topic: str = "volume-test"):
        """Test handling of large data volumes"""
        print(f"\n📈 Testing large data volume: {num_messages} messages of {message_size_kb}KB each")
        
        self.setup_topics([topic])
        
        # Create subscriber
        subscriber = WebSocketClient(self.ws_url, "volume-subscriber")
        if not subscriber.connect():
            print("Failed to connect subscriber")
            return None
        
        subscriber.subscribe(topic)
        time.sleep(0.5)
        
        # Create publisher
        publisher = WebSocketClient(self.ws_url, "volume-publisher")
        if not publisher.connect():
            print("Failed to connect publisher")
            subscriber.close()
            return None
        
        # Generate large payload
        large_data = "x" * (message_size_kb * 1024)  # KB to bytes
        
        # Publish large messages
        start_time = time.time()
        messages_sent = 0
        
        for i in range(num_messages):
            payload = {
                "message_id": i,
                "timestamp": time.time(),
                "large_data": large_data,
                "metadata": {
                    "size_kb": message_size_kb,
                    "sequence": i,
                    "test_type": "volume"
                }
            }
            
            if publisher.publish(topic, payload):
                messages_sent += 1
            
            # Small delay to avoid overwhelming the server
            if i % 100 == 0:
                time.sleep(0.01)
        
        # Wait for message propagation
        time.sleep(5)
        
        end_time = time.time()
        duration = end_time - start_time
        
        # Collect results
        with subscriber.lock:
            messages_received = len(subscriber.messages_received)
            latencies = [latency for _, latency in subscriber.messages_received]
        
        total_data_mb = (messages_sent * message_size_kb) / 1024
        throughput_mbps = total_data_mb / duration if duration > 0 else 0
        
        print(f"📊 Large Data Volume Test Results:")
        print(f"   Duration: {duration:.2f}s")
        print(f"   Messages sent: {messages_sent}")
        print(f"   Messages received: {messages_received}")
        print(f"   Success rate: {(messages_received/messages_sent)*100:.1f}%" if messages_sent > 0 else "N/A")
        print(f"   Total data: {total_data_mb:.2f} MB")
        print(f"   Throughput: {throughput_mbps:.2f} MB/s")
        print(f"   Avg latency: {statistics.mean(latencies):.3f}s" if latencies else "N/A")
        
        # Cleanup
        publisher.close()
        subscriber.close()
        self.cleanup_topics([topic])
        
        return {
            "messages_sent": messages_sent,
            "messages_received": messages_received,
            "duration": duration,
            "throughput_mbps": throughput_mbps,
            "latencies": latencies
        }
    
    def test_ring_buffer(self, buffer_size: int = 100, messages_to_send: int = 150, 
                        topic: str = "ringbuffer-test"):
        """Test ring buffer functionality with historical message replay"""
        print(f"\n🔄 Testing ring buffer: sending {messages_to_send} messages, buffer size ~{buffer_size}")
        
        self.setup_topics([topic])
        
        # Create publisher first
        publisher = WebSocketClient(self.ws_url, "ringbuffer-publisher")
        if not publisher.connect():
            print("Failed to connect publisher")
            return None
        
        # Publish messages before subscriber connects
        print(f"Publishing {messages_to_send} messages...")
        for i in range(messages_to_send):
            payload = {
                "sequence": i,
                "timestamp": time.time(),
                "data": f"Ring buffer test message {i}",
                "test_phase": "pre-subscriber"
            }
            publisher.publish(topic, payload)
            time.sleep(0.01)  # Small delay
        
        # Wait for messages to be processed
        time.sleep(2)
        
        # Test different last_n values
        test_cases = [10, 50, buffer_size, messages_to_send + 10]  # Last one tests upper bound
        
        results = {}
        
        for last_n in test_cases:
            print(f"Testing last_n={last_n}...")
            
            # Create subscriber with last_n
            subscriber = WebSocketClient(self.ws_url, f"ringbuffer-subscriber-{last_n}")
            if not subscriber.connect():
                print(f"Failed to connect subscriber for last_n={last_n}")
                continue
            
            subscriber.subscribe(topic, last_n=last_n)
            
            # Wait for historical messages
            time.sleep(2)
            
            # Send a few more messages to test real-time delivery
            for i in range(5):
                payload = {
                    "sequence": messages_to_send + i,
                    "timestamp": time.time(),
                    "data": f"Real-time message {i}",
                    "test_phase": "post-subscriber"
                }
                publisher.publish(topic, payload)
                time.sleep(0.1)
            
            # Wait for real-time messages
            time.sleep(1)
            
            # Collect results
            with subscriber.lock:
                received_messages = [msg for msg, _ in subscriber.messages_received]
                historical_messages = [
                    msg for msg in received_messages 
                    if msg.get("message", {}).get("payload", {}).get("test_phase") == "pre-subscriber"
                ]
                realtime_messages = [
                    msg for msg in received_messages 
                    if msg.get("message", {}).get("payload", {}).get("test_phase") == "post-subscriber"
                ]
            
            expected_historical = min(last_n, messages_to_send)
            
            print(f"   last_n={last_n}: Expected historical: {expected_historical}, "
                  f"Received historical: {len(historical_messages)}, "
                  f"Received real-time: {len(realtime_messages)}")
            
            results[last_n] = {
                "expected_historical": expected_historical,
                "received_historical": len(historical_messages),
                "received_realtime": len(realtime_messages),
                "total_received": len(received_messages)
            }
            
            subscriber.close()
            time.sleep(0.5)
        
        # Cleanup
        publisher.close()
        self.cleanup_topics([topic])
        
        print(f"📊 Ring Buffer Test Results:")
        for last_n, result in results.items():
            accuracy = (result["received_historical"] / result["expected_historical"] * 100) if result["expected_historical"] > 0 else 0
            print(f"   last_n={last_n}: {accuracy:.1f}% accuracy "
                  f"({result['received_historical']}/{result['expected_historical']} historical, "
                  f"{result['received_realtime']} real-time)")
        
        return results
    
    def test_backpressure_and_dropping(self, buffer_size: int = 10, burst_size: int = 50, 
                                     topic: str = "backpressure-test"):
        """Test backpressure handling and message dropping policies"""
        print(f"\n⚡ Testing backpressure: burst of {burst_size} messages, buffer size ~{buffer_size}")
        
        self.setup_topics([topic])
        
        # Create slow subscriber (simulate slow processing)
        subscriber = WebSocketClient(self.ws_url, "slow-subscriber")
        if not subscriber.connect():
            print("Failed to connect subscriber")
            return None
        
        subscriber.subscribe(topic)
        time.sleep(0.5)
        
        # Create fast publisher
        publisher = WebSocketClient(self.ws_url, "fast-publisher")
        if not publisher.connect():
            print("Failed to connect publisher")
            subscriber.close()
            return None
        
        # Send burst of messages rapidly
        start_time = time.time()
        messages_sent = 0
        
        for i in range(burst_size):
            payload = {
                "sequence": i,
                "timestamp": time.time(),
                "data": f"Burst message {i}",
                "burst_test": True
            }
            
            if publisher.publish(topic, payload):
                messages_sent += 1
            
            # No delay - send as fast as possible to trigger backpressure
        
        # Wait for processing
        time.sleep(3)
        
        # Collect results
        with subscriber.lock:
            messages_received = len(subscriber.messages_received)
            received_sequences = [
                msg.get("message", {}).get("payload", {}).get("sequence", -1)
                for msg, _ in subscriber.messages_received
            ]
        
        # Analyze dropping pattern
        received_sequences = [seq for seq in received_sequences if seq >= 0]
        received_sequences.sort()
        
        # Check if oldest messages were dropped (DROP_OLDEST policy)
        expected_received = min(messages_sent, buffer_size)
        if len(received_sequences) > 0:
            first_received = min(received_sequences)
            last_received = max(received_sequences)
            expected_first = max(0, burst_size - buffer_size)  # Should drop oldest
        else:
            first_received = last_received = expected_first = -1
        
        print(f"📊 Backpressure Test Results:")
        print(f"   Messages sent: {messages_sent}")
        print(f"   Messages received: {messages_received}")
        print(f"   Expected received (max buffer): {expected_received}")
        print(f"   First sequence received: {first_received}")
        print(f"   Last sequence received: {last_received}")
        print(f"   Expected first (DROP_OLDEST): {expected_first}")
        print(f"   DROP_OLDEST working: {first_received >= expected_first}")
        
        # Cleanup
        publisher.close()
        subscriber.close()
        self.cleanup_topics([topic])
        
        return {
            "messages_sent": messages_sent,
            "messages_received": messages_received,
            "drop_oldest_working": first_received >= expected_first,
            "first_received": first_received,
            "expected_first": expected_first
        }
    
    def run_comprehensive_test(self):
        """Run all tests in sequence"""
        print("🚀 Starting Comprehensive Stress Test Suite")
        print("=" * 60)
        
        # Check service health
        try:
            response = requests.get(f"{self.base_url}/health", timeout=5)
            if response.status_code != 200:
                print(f"❌ Service health check failed: {response.status_code}")
                return
            print("✅ Service is healthy")
        except Exception as e:
            print(f"❌ Cannot connect to service: {e}")
            return
        
        all_results = {}
        
        try:
            # Test 1: Race Conditions
            all_results["race_conditions"] = self.test_race_conditions(
                num_publishers=5, num_subscribers=5, messages_per_publisher=50
            )
            
            # Test 2: Large Data Volume
            all_results["large_data_volume"] = self.test_large_data_volume(
                message_size_kb=5, num_messages=200
            )
            
            # Test 3: Ring Buffer
            all_results["ring_buffer"] = self.test_ring_buffer(
                buffer_size=50, messages_to_send=75
            )
            
            # Test 4: Backpressure
            all_results["backpressure"] = self.test_backpressure_and_dropping(
                buffer_size=20, burst_size=50
            )
            
        except KeyboardInterrupt:
            print("\n⚠️  Test interrupted by user")
        except Exception as e:
            print(f"\n❌ Test failed with error: {e}")
        
        # Summary
        print("\n" + "=" * 60)
        print("📋 TEST SUMMARY")
        print("=" * 60)
        
        for test_name, result in all_results.items():
            if result:
                print(f"\n{test_name.upper().replace('_', ' ')}:")
                if isinstance(result, dict):
                    for key, value in result.items():
                        if isinstance(value, (int, float)):
                            print(f"  {key}: {value}")
                        elif isinstance(value, bool):
                            print(f"  {key}: {'✅' if value else '❌'}")
        
        return all_results

def signal_handler(signum, frame):
    """Handle Ctrl+C gracefully"""
    print("\n⚠️  Received interrupt signal. Cleaning up...")
    sys.exit(0)

if __name__ == "__main__":
    # Set up signal handler for graceful shutdown
    signal.signal(signal.SIGINT, signal_handler)
    
    # Run the comprehensive test suite
    test_suite = StressTestSuite()
    results = test_suite.run_comprehensive_test()
    
    print("\n🎉 Test suite completed!")
