#!/usr/bin/env python3
"""
Quick stress test for inmem-pubsub service.
Lightweight version for rapid testing.
"""

import json
import time
import uuid
import threading
import websocket
import requests
from concurrent.futures import ThreadPoolExecutor
import statistics

class QuickStressTest:
    def __init__(self, base_url="http://localhost:8080", ws_url="ws://localhost:8080/ws"):
        self.base_url = base_url
        self.ws_url = ws_url
        
    def create_topic(self, topic):
        """Create a topic via REST API"""
        try:
            response = requests.post(f"{self.base_url}/topics", json={"name": topic})
            return response.status_code in [201, 409]
        except:
            return False
            
    def delete_topic(self, topic):
        """Delete a topic via REST API"""
        try:
            requests.delete(f"{self.base_url}/topics/{topic}")
        except:
            pass

    def test_concurrent_publishers(self, num_publishers=5, messages_each=20):
        """Test with multiple concurrent publishers"""
        print(f"🔥 Testing {num_publishers} concurrent publishers, {messages_each} messages each")
        
        topic = "concurrent-test"
        self.create_topic(topic)
        
        # Results tracking
        results = {"sent": 0, "received": 0, "latencies": []}
        lock = threading.Lock()
        
        # Create subscriber
        subscriber_messages = []
        
        def on_message(ws, message):
            try:
                msg = json.loads(message)
                if msg.get("type") == "event":
                    with lock:
                        subscriber_messages.append(msg)
            except:
                pass
        
        # Connect subscriber
        subscriber_ws = websocket.WebSocketApp(
            self.ws_url,
            on_message=on_message
        )
        
        subscriber_thread = threading.Thread(target=subscriber_ws.run_forever)
        subscriber_thread.daemon = True
        subscriber_thread.start()
        
        time.sleep(1)  # Wait for connection
        
        # Subscribe to topic
        subscribe_msg = {
            "type": "subscribe",
            "topic": topic,
            "client_id": "test-subscriber"
        }
        subscriber_ws.send(json.dumps(subscribe_msg))
        time.sleep(0.5)
        
        # Publisher function
        def publisher_worker(publisher_id):
            sent_count = 0
            try:
                # Connect publisher
                publisher_ws = websocket.create_connection(self.ws_url)
                
                for i in range(messages_each):
                    message = {
                        "type": "publish",
                        "topic": topic,
                        "message": {
                            "id": str(uuid.uuid4()),
                            "payload": {
                                "publisher_id": publisher_id,
                                "message_num": i,
                                "timestamp": time.time()
                            }
                        }
                    }
                    
                    start_time = time.time()
                    publisher_ws.send(json.dumps(message))
                    sent_count += 1
                    
                    # Small random delay
                    time.sleep(0.01 + (i % 5) * 0.001)
                
                publisher_ws.close()
                
            except Exception as e:
                print(f"Publisher {publisher_id} error: {e}")
            
            with lock:
                results["sent"] += sent_count
            
            return sent_count
        
        # Run concurrent publishers
        start_time = time.time()
        
        with ThreadPoolExecutor(max_workers=num_publishers) as executor:
            futures = [executor.submit(publisher_worker, i) for i in range(num_publishers)]
            for future in futures:
                future.result()
        
        # Wait for message propagation
        time.sleep(2)
        
        duration = time.time() - start_time
        
        # Close subscriber
        subscriber_ws.close()
        
        with lock:
            received_count = len(subscriber_messages)
            results["received"] = received_count
        
        # Calculate metrics
        throughput = received_count / duration if duration > 0 else 0
        expected = results["sent"]
        success_rate = (received_count / expected * 100) if expected > 0 else 0
        
        print(f"📊 Results:")
        print(f"   Duration: {duration:.2f}s")
        print(f"   Messages sent: {results['sent']}")
        print(f"   Messages received: {received_count}")
        print(f"   Success rate: {success_rate:.1f}%")
        print(f"   Throughput: {throughput:.1f} msg/s")
        
        self.delete_topic(topic)
        return results

    def test_ring_buffer_quick(self, pre_messages=30, last_n=10):
        """Quick ring buffer test"""
        print(f"🔄 Quick ring buffer test: {pre_messages} pre-messages, last_n={last_n}")
        
        topic = "ringbuffer-quick"
        self.create_topic(topic)
        
        # Send messages before subscriber
        publisher_ws = websocket.create_connection(self.ws_url)
        
        for i in range(pre_messages):
            message = {
                "type": "publish",
                "topic": topic,
                "message": {
                    "id": str(uuid.uuid4()),
                    "payload": {"sequence": i, "data": f"Pre-message {i}"}
                }
            }
            publisher_ws.send(json.dumps(message))
            time.sleep(0.01)
        
        time.sleep(1)  # Let messages settle
        
        # Connect subscriber with last_n
        received_messages = []
        
        def on_message(ws, message):
            try:
                msg = json.loads(message)
                if msg.get("type") == "event":
                    received_messages.append(msg)
            except:
                pass
        
        subscriber_ws = websocket.WebSocketApp(
            self.ws_url,
            on_message=on_message
        )
        
        subscriber_thread = threading.Thread(target=subscriber_ws.run_forever)
        subscriber_thread.daemon = True
        subscriber_thread.start()
        
        time.sleep(1)
        
        # Subscribe with last_n
        subscribe_msg = {
            "type": "subscribe",
            "topic": topic,
            "client_id": "ringbuffer-subscriber",
            "last_n": last_n
        }
        subscriber_ws.send(json.dumps(subscribe_msg))
        
        # Wait for historical messages
        time.sleep(2)
        
        historical_count = len(received_messages)
        expected = min(last_n, pre_messages)
        
        print(f"📊 Ring Buffer Results:")
        print(f"   Pre-messages sent: {pre_messages}")
        print(f"   Requested last_n: {last_n}")
        print(f"   Expected historical: {expected}")
        print(f"   Received historical: {historical_count}")
        print(f"   Accuracy: {(historical_count/expected*100):.1f}%" if expected > 0 else "N/A")
        
        # Cleanup
        publisher_ws.close()
        subscriber_ws.close()
        self.delete_topic(topic)
        
        return {
            "expected": expected,
            "received": historical_count,
            "accuracy": (historical_count/expected) if expected > 0 else 0
        }

    def test_large_message(self, message_size_kb=50, num_messages=10):
        """Test large message handling"""
        print(f"📈 Testing large messages: {num_messages} messages of {message_size_kb}KB")
        
        topic = "large-message-test"
        self.create_topic(topic)
        
        # Create large payload
        large_data = "x" * (message_size_kb * 1024)
        
        received_count = 0
        latencies = []
        
        def on_message(ws, message):
            nonlocal received_count
            try:
                msg = json.loads(message)
                if msg.get("type") == "event":
                    received_count += 1
                    # Calculate approximate latency (simple estimation)
                    latencies.append(0.1)  # Placeholder
            except:
                pass
        
        # Connect subscriber
        subscriber_ws = websocket.WebSocketApp(
            self.ws_url,
            on_message=on_message
        )
        
        subscriber_thread = threading.Thread(target=subscriber_ws.run_forever)
        subscriber_thread.daemon = True
        subscriber_thread.start()
        
        time.sleep(1)
        
        # Subscribe
        subscribe_msg = {
            "type": "subscribe",
            "topic": topic,
            "client_id": "large-message-subscriber"
        }
        subscriber_ws.send(json.dumps(subscribe_msg))
        time.sleep(0.5)
        
        # Connect publisher and send large messages
        publisher_ws = websocket.create_connection(self.ws_url)
        
        start_time = time.time()
        sent_count = 0
        
        for i in range(num_messages):
            message = {
                "type": "publish",
                "topic": topic,
                "message": {
                    "id": str(uuid.uuid4()),
                    "payload": {
                        "sequence": i,
                        "large_data": large_data,
                        "metadata": {"size_kb": message_size_kb}
                    }
                }
            }
            
            try:
                publisher_ws.send(json.dumps(message))
                sent_count += 1
                time.sleep(0.1)  # Small delay between large messages
            except Exception as e:
                print(f"Failed to send large message {i}: {e}")
        
        # Wait for processing
        time.sleep(3)
        
        duration = time.time() - start_time
        total_data_mb = (sent_count * message_size_kb) / 1024
        throughput_mbps = total_data_mb / duration if duration > 0 else 0
        
        print(f"📊 Large Message Results:")
        print(f"   Messages sent: {sent_count}")
        print(f"   Messages received: {received_count}")
        print(f"   Success rate: {(received_count/sent_count*100):.1f}%" if sent_count > 0 else "N/A")
        print(f"   Total data: {total_data_mb:.2f} MB")
        print(f"   Throughput: {throughput_mbps:.2f} MB/s")
        
        # Cleanup
        publisher_ws.close()
        subscriber_ws.close()
        self.delete_topic(topic)
        
        return {
            "sent": sent_count,
            "received": received_count,
            "throughput_mbps": throughput_mbps
        }

    def run_all_tests(self):
        """Run all quick tests"""
        print("🚀 Running Quick Stress Tests")
        print("=" * 40)
        
        # Check service health
        try:
            response = requests.get(f"{self.base_url}/health", timeout=5)
            if response.status_code != 200:
                print(f"❌ Service not healthy: {response.status_code}")
                return
            print("✅ Service is healthy\n")
        except Exception as e:
            print(f"❌ Cannot connect to service: {e}")
            return
        
        results = {}
        
        try:
            # Test 1: Concurrent publishers
            results["concurrent"] = self.test_concurrent_publishers(
                num_publishers=3, messages_each=15
            )
            print()
            
            # Test 2: Ring buffer
            results["ring_buffer"] = self.test_ring_buffer_quick(
                pre_messages=25, last_n=10
            )
            print()
            
            # Test 3: Large messages
            results["large_messages"] = self.test_large_message(
                message_size_kb=20, num_messages=5
            )
            print()
            
        except KeyboardInterrupt:
            print("⚠️  Tests interrupted")
        except Exception as e:
            print(f"❌ Test error: {e}")
        
        print("=" * 40)
        print("🎉 Quick stress tests completed!")
        
        return results

if __name__ == "__main__":
    test = QuickStressTest()
    test.run_all_tests()
