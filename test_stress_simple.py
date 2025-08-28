#!/usr/bin/env python3
"""
Simple stress test for inmem-pubsub service using synchronous approach.
Tests race conditions, large data volume, and ring buffer functionality.
"""

import json
import time
import uuid
import threading
import websocket
import requests
from concurrent.futures import ThreadPoolExecutor, as_completed
import statistics
import queue

def create_topic(base_url, topic):
    """Create a topic via REST API"""
    try:
        response = requests.post(f"{base_url}/topics", json={"name": topic})
        return response.status_code in [201, 409]
    except:
        return False

def delete_topic(base_url, topic):
    """Delete a topic via REST API"""
    try:
        requests.delete(f"{base_url}/topics/{topic}")
    except:
        pass

def test_concurrent_publishing(base_url="http://localhost:8080", ws_url="ws://localhost:8080/ws"):
    """Test concurrent publishing with multiple clients"""
    print("🔥 Testing concurrent publishing...")
    
    topic = f"concurrent-test-{int(time.time())}"
    create_topic(base_url, topic)
    
    num_publishers = 3
    messages_per_publisher = 10
    total_expected = num_publishers * messages_per_publisher
    
    # Subscriber to collect all messages
    subscriber_messages = queue.Queue()
    subscriber_responses = queue.Queue()
    
    def on_message(ws, message):
        try:
            msg = json.loads(message)
            subscriber_responses.put(msg)
            if msg.get("type") == "event":
                subscriber_messages.put(msg)
        except Exception as e:
            print(f"Subscriber message parsing error: {e}")
    
    def on_error(ws, error):
        print(f"Subscriber WebSocket error: {error}")
    
    # Create subscriber
    subscriber_ws = websocket.WebSocketApp(
        ws_url,
        on_message=on_message,
        on_error=on_error
    )
    
    # Start subscriber in background
    subscriber_thread = threading.Thread(target=subscriber_ws.run_forever)
    subscriber_thread.daemon = True
    subscriber_thread.start()
    
    time.sleep(1)  # Wait for connection
    
    # Subscribe to topic
    subscribe_msg = {
        "type": "subscribe",
        "topic": topic,
        "client_id": "stress-subscriber",
        "request_id": str(uuid.uuid4())
    }
    subscriber_ws.send(json.dumps(subscribe_msg))
    time.sleep(1)  # Wait for subscription
    
    # Publisher function
    def publisher_worker(publisher_id):
        messages_sent = 0
        try:
            # Create connection
            pub_ws = websocket.create_connection(ws_url, timeout=10)
            
            for i in range(messages_per_publisher):
                message = {
                    "type": "publish",
                    "topic": topic,
                    "message": {
                        "id": str(uuid.uuid4()),
                        "payload": {
                            "publisher_id": publisher_id,
                            "message_num": i,
                            "timestamp": time.time(),
                            "data": f"Message {i} from publisher {publisher_id}"
                        }
                    },
                    "request_id": str(uuid.uuid4())
                }
                
                pub_ws.send(json.dumps(message))
                messages_sent += 1
                
                # Small delay to create realistic conditions
                time.sleep(0.05)
            
            pub_ws.close()
            
        except Exception as e:
            print(f"Publisher {publisher_id} error: {e}")
        
        return messages_sent
    
    # Run publishers concurrently
    start_time = time.time()
    total_sent = 0
    
    with ThreadPoolExecutor(max_workers=num_publishers) as executor:
        futures = [executor.submit(publisher_worker, i) for i in range(num_publishers)]
        for future in as_completed(futures):
            total_sent += future.result()
    
    # Wait for message propagation
    time.sleep(3)
    
    duration = time.time() - start_time
    
    # Collect results
    received_messages = []
    while not subscriber_messages.empty():
        received_messages.append(subscriber_messages.get())
    
    # Close subscriber
    subscriber_ws.close()
    
    success_rate = (len(received_messages) / total_expected * 100) if total_expected > 0 else 0
    throughput = len(received_messages) / duration if duration > 0 else 0
    
    print(f"📊 Concurrent Publishing Results:")
    print(f"   Publishers: {num_publishers}")
    print(f"   Messages per publisher: {messages_per_publisher}")
    print(f"   Total sent: {total_sent}")
    print(f"   Total received: {len(received_messages)}")
    print(f"   Success rate: {success_rate:.1f}%")
    print(f"   Throughput: {throughput:.1f} msg/s")
    print(f"   Duration: {duration:.2f}s")
    
    delete_topic(base_url, topic)
    
    return {
        "sent": total_sent,
        "received": len(received_messages),
        "success_rate": success_rate,
        "throughput": throughput
    }

def test_ring_buffer(base_url="http://localhost:8080", ws_url="ws://localhost:8080/ws"):
    """Test ring buffer functionality"""
    print("🔄 Testing ring buffer functionality...")
    
    topic = f"ringbuffer-test-{int(time.time())}"
    create_topic(base_url, topic)
    
    # Send messages before subscriber connects
    pre_messages = 15
    last_n = 8
    
    # Publisher connection
    pub_ws = websocket.create_connection(ws_url, timeout=10)
    
    print(f"   Sending {pre_messages} messages before subscriber...")
    for i in range(pre_messages):
        message = {
            "type": "publish",
            "topic": topic,
            "message": {
                "id": str(uuid.uuid4()),
                "payload": {
                    "sequence": i,
                    "data": f"Pre-message {i}",
                    "timestamp": time.time()
                }
            },
            "request_id": str(uuid.uuid4())
        }
        pub_ws.send(json.dumps(message))
        time.sleep(0.02)
    
    time.sleep(2)  # Let messages settle
    
    # Create subscriber with last_n
    subscriber_messages = queue.Queue()
    historical_count = 0
    realtime_count = 0
    
    def on_message(ws, message):
        nonlocal historical_count, realtime_count
        try:
            msg = json.loads(message)
            if msg.get("type") == "event":
                payload = msg.get("message", {}).get("payload", {})
                if "sequence" in payload:
                    sequence = payload["sequence"]
                    if sequence < pre_messages:
                        historical_count += 1
                    else:
                        realtime_count += 1
                    subscriber_messages.put(msg)
        except Exception as e:
            print(f"Ring buffer message parsing error: {e}")
    
    subscriber_ws = websocket.WebSocketApp(
        ws_url,
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
        "last_n": last_n,
        "request_id": str(uuid.uuid4())
    }
    subscriber_ws.send(json.dumps(subscribe_msg))
    
    time.sleep(2)  # Wait for historical messages
    
    # Send real-time messages
    print(f"   Sending 3 real-time messages...")
    for i in range(3):
        message = {
            "type": "publish",
            "topic": topic,
            "message": {
                "id": str(uuid.uuid4()),
                "payload": {
                    "sequence": pre_messages + i,
                    "data": f"Real-time message {i}",
                    "timestamp": time.time()
                }
            },
            "request_id": str(uuid.uuid4())
        }
        pub_ws.send(json.dumps(message))
        time.sleep(0.1)
    
    time.sleep(2)  # Wait for real-time messages
    
    # Close connections
    pub_ws.close()
    subscriber_ws.close()
    
    expected_historical = min(last_n, pre_messages)
    expected_realtime = 3
    
    accuracy = (historical_count / expected_historical * 100) if expected_historical > 0 else 0
    
    print(f"📊 Ring Buffer Results:")
    print(f"   Pre-messages sent: {pre_messages}")
    print(f"   Requested last_n: {last_n}")
    print(f"   Expected historical: {expected_historical}")
    print(f"   Received historical: {historical_count}")
    print(f"   Expected real-time: {expected_realtime}")
    print(f"   Received real-time: {realtime_count}")
    print(f"   Historical accuracy: {accuracy:.1f}%")
    
    delete_topic(base_url, topic)
    
    return {
        "expected_historical": expected_historical,
        "received_historical": historical_count,
        "expected_realtime": expected_realtime,
        "received_realtime": realtime_count,
        "accuracy": accuracy
    }

def test_large_messages(base_url="http://localhost:8080", ws_url="ws://localhost:8080/ws"):
    """Test large message handling"""
    print("📈 Testing large message handling...")
    
    topic = f"large-test-{int(time.time())}"
    create_topic(base_url, topic)
    
    message_size_kb = 10
    num_messages = 5
    large_data = "x" * (message_size_kb * 1024)
    
    # Subscriber setup
    received_count = 0
    latencies = []
    
    def on_message(ws, message):
        nonlocal received_count
        try:
            msg = json.loads(message)
            if msg.get("type") == "event":
                received_count += 1
        except Exception as e:
            print(f"Large message parsing error: {e}")
    
    subscriber_ws = websocket.WebSocketApp(
        ws_url,
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
        "client_id": "large-subscriber",
        "request_id": str(uuid.uuid4())
    }
    subscriber_ws.send(json.dumps(subscribe_msg))
    time.sleep(1)
    
    # Publisher connection
    pub_ws = websocket.create_connection(ws_url, timeout=15)
    
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
            },
            "request_id": str(uuid.uuid4())
        }
        
        try:
            pub_ws.send(json.dumps(message))
            sent_count += 1
            time.sleep(0.2)  # Delay between large messages
        except Exception as e:
            print(f"Failed to send large message {i}: {e}")
    
    time.sleep(3)  # Wait for processing
    
    duration = time.time() - start_time
    total_data_mb = (sent_count * message_size_kb) / 1024
    throughput_mbps = total_data_mb / duration if duration > 0 else 0
    
    # Close connections
    pub_ws.close()
    subscriber_ws.close()
    
    success_rate = (received_count / sent_count * 100) if sent_count > 0 else 0
    
    print(f"📊 Large Message Results:")
    print(f"   Message size: {message_size_kb} KB")
    print(f"   Messages sent: {sent_count}")
    print(f"   Messages received: {received_count}")
    print(f"   Success rate: {success_rate:.1f}%")
    print(f"   Total data: {total_data_mb:.2f} MB")
    print(f"   Throughput: {throughput_mbps:.2f} MB/s")
    print(f"   Duration: {duration:.2f}s")
    
    delete_topic(base_url, topic)
    
    return {
        "sent": sent_count,
        "received": received_count,
        "success_rate": success_rate,
        "throughput_mbps": throughput_mbps
    }

def test_backpressure(base_url="http://localhost:8080", ws_url="ws://localhost:8080/ws"):
    """Test backpressure and message dropping"""
    print("⚡ Testing backpressure handling...")
    
    topic = f"backpressure-test-{int(time.time())}"
    create_topic(base_url, topic)
    
    burst_size = 25
    
    # Slow subscriber (doesn't read messages quickly)
    received_messages = []
    
    def on_message(ws, message):
        try:
            msg = json.loads(message)
            if msg.get("type") == "event":
                received_messages.append(msg)
        except Exception as e:
            print(f"Backpressure message parsing error: {e}")
    
    subscriber_ws = websocket.WebSocketApp(
        ws_url,
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
        "client_id": "backpressure-subscriber",
        "request_id": str(uuid.uuid4())
    }
    subscriber_ws.send(json.dumps(subscribe_msg))
    time.sleep(1)
    
    # Fast publisher - send burst of messages
    pub_ws = websocket.create_connection(ws_url, timeout=10)
    
    sent_count = 0
    for i in range(burst_size):
        message = {
            "type": "publish",
            "topic": topic,
            "message": {
                "id": str(uuid.uuid4()),
                "payload": {
                    "sequence": i,
                    "data": f"Burst message {i}",
                    "timestamp": time.time()
                }
            },
            "request_id": str(uuid.uuid4())
        }
        
        try:
            pub_ws.send(json.dumps(message))
            sent_count += 1
            # No delay - send as fast as possible
        except Exception as e:
            print(f"Failed to send burst message {i}: {e}")
    
    time.sleep(3)  # Wait for processing
    
    # Analyze dropping pattern
    received_sequences = []
    for msg in received_messages:
        payload = msg.get("message", {}).get("payload", {})
        if "sequence" in payload:
            received_sequences.append(payload["sequence"])
    
    received_sequences.sort()
    received_count = len(received_sequences)
    
    # Check if oldest messages were dropped (DROP_OLDEST policy)
    drop_oldest_working = True
    if received_count > 0 and received_count < sent_count:
        # Should have kept the newest messages
        expected_first = sent_count - received_count
        actual_first = min(received_sequences) if received_sequences else -1
        drop_oldest_working = actual_first >= expected_first
    
    # Close connections
    pub_ws.close()
    subscriber_ws.close()
    
    print(f"📊 Backpressure Results:")
    print(f"   Messages sent: {sent_count}")
    print(f"   Messages received: {received_count}")
    print(f"   Sequences received: {received_sequences[:10]}{'...' if len(received_sequences) > 10 else ''}")
    print(f"   DROP_OLDEST working: {'✅' if drop_oldest_working else '❌'}")
    
    delete_topic(base_url, topic)
    
    return {
        "sent": sent_count,
        "received": received_count,
        "drop_oldest_working": drop_oldest_working,
        "sequences": received_sequences
    }

def main():
    """Run all stress tests"""
    print("🚀 Starting Simple Stress Test Suite")
    print("=" * 50)
    
    base_url = "http://localhost:8080"
    ws_url = "ws://localhost:8080/ws"
    
    # Check service health
    try:
        response = requests.get(f"{base_url}/health", timeout=5)
        if response.status_code != 200:
            print(f"❌ Service not healthy: {response.status_code}")
            return
        print("✅ Service is healthy\n")
    except Exception as e:
        print(f"❌ Cannot connect to service: {e}")
        return
    
    results = {}
    
    try:
        # Test 1: Concurrent Publishing
        results["concurrent"] = test_concurrent_publishing(base_url, ws_url)
        print()
        
        # Test 2: Ring Buffer
        results["ring_buffer"] = test_ring_buffer(base_url, ws_url)
        print()
        
        # Test 3: Large Messages
        results["large_messages"] = test_large_messages(base_url, ws_url)
        print()
        
        # Test 4: Backpressure
        results["backpressure"] = test_backpressure(base_url, ws_url)
        print()
        
    except KeyboardInterrupt:
        print("⚠️  Tests interrupted by user")
    except Exception as e:
        print(f"❌ Test error: {e}")
        import traceback
        traceback.print_exc()
    
    # Summary
    print("=" * 50)
    print("📋 SUMMARY")
    print("=" * 50)
    
    for test_name, result in results.items():
        if result:
            print(f"\n{test_name.upper().replace('_', ' ')}:")
            for key, value in result.items():
                if isinstance(value, (int, float)) and key != "sequences":
                    print(f"  {key}: {value}")
                elif isinstance(value, bool):
                    print(f"  {key}: {'✅' if value else '❌'}")
    
    print("\n🎉 Stress tests completed!")

if __name__ == "__main__":
    main()
