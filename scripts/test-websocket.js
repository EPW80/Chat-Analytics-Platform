#!/usr/bin/env node

const WebSocket = require('ws');

// Colors for terminal output
const colors = {
  reset: '\x1b[0m',
  green: '\x1b[32m',
  blue: '\x1b[34m',
  yellow: '\x1b[33m',
  red: '\x1b[31m',
  cyan: '\x1b[36m',
};

function log(color, prefix, message) {
  console.log(`${color}[${prefix}]${colors.reset} ${message}`);
}

function createClient(userId, username) {
  return new Promise((resolve, reject) => {
    const url = `ws://localhost:8080/ws?userId=${userId}&username=${username}`;
    const ws = new WebSocket(url);
    const messages = [];

    ws.on('open', () => {
      log(colors.green, username, 'Connected');
      resolve({ ws, messages, userId, username });
    });

    ws.on('message', (data) => {
      try {
        const msg = JSON.parse(data.toString());
        messages.push(msg);
        log(colors.blue, username, `Received: ${msg.type} - ${msg.content || 'N/A'}`);
      } catch (err) {
        log(colors.red, username, `Error parsing message: ${err.message}`);
      }
    });

    ws.on('error', (error) => {
      log(colors.red, username, `Error: ${error.message}`);
      reject(error);
    });

    ws.on('close', () => {
      log(colors.yellow, username, 'Disconnected');
    });
  });
}

function sendMessage(client, type, content) {
  return new Promise((resolve) => {
    const message = { type, content };
    client.ws.send(JSON.stringify(message));
    log(colors.cyan, client.username, `Sent: ${type} - ${content}`);
    // Wait a bit for the message to be broadcast
    setTimeout(resolve, 100);
  });
}

function wait(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

async function runTests() {
  console.log('\n' + '='.repeat(60));
  console.log('WebSocket Chat Platform - Functionality Test');
  console.log('='.repeat(60) + '\n');

  let testsPassed = 0;
  let testsFailed = 0;

  try {
    // Test 1: Connect multiple clients
    console.log('\n--- Test 1: Multi-client Connection ---');
    const alice = await createClient('alice', 'Alice');
    const bob = await createClient('bob', 'Bob');
    const charlie = await createClient('charlie', 'Charlie');
    testsPassed++;
    log(colors.green, 'TEST', 'Multi-client connection: PASSED');

    await wait(500);

    // Test 2: Send chat messages
    console.log('\n--- Test 2: Chat Messages ---');
    await sendMessage(alice, 'chat', 'Hello everyone!');
    await wait(300);
    await sendMessage(bob, 'chat', 'Hi Alice!');
    await wait(300);
    await sendMessage(charlie, 'chat', 'Hey folks!');
    await wait(500);

    // Verify messages were received
    if (bob.messages.length >= 2 && charlie.messages.length >= 2) {
      testsPassed++;
      log(colors.green, 'TEST', 'Message broadcasting: PASSED');
    } else {
      testsFailed++;
      log(colors.red, 'TEST', `Message broadcasting: FAILED (Bob: ${bob.messages.length}, Charlie: ${charlie.messages.length})`);
    }

    // Test 3: System messages
    console.log('\n--- Test 3: System Messages ---');
    await sendMessage(alice, 'system', 'Server maintenance in 5 minutes');
    await wait(500);

    if (bob.messages.some(m => m.type === 'system')) {
      testsPassed++;
      log(colors.green, 'TEST', 'System messages: PASSED');
    } else {
      testsFailed++;
      log(colors.red, 'TEST', 'System messages: FAILED');
    }

    // Test 4: Message validation (long content)
    console.log('\n--- Test 4: Message Validation ---');
    const longMessage = 'A'.repeat(500);
    await sendMessage(bob, 'chat', longMessage);
    await wait(500);

    if (alice.messages.some(m => m.content === longMessage)) {
      testsPassed++;
      log(colors.green, 'TEST', 'Long message handling: PASSED');
    } else {
      testsFailed++;
      log(colors.red, 'TEST', 'Long message handling: FAILED');
    }

    // Test 5: Verify message structure
    console.log('\n--- Test 5: Message Structure ---');
    const sampleMsg = alice.messages[0];
    const hasRequiredFields = sampleMsg.messageId &&
                              sampleMsg.userId &&
                              sampleMsg.username &&
                              sampleMsg.timestamp &&
                              sampleMsg.type;

    if (hasRequiredFields) {
      testsPassed++;
      log(colors.green, 'TEST', 'Message structure validation: PASSED');
      console.log('Sample message:', JSON.stringify(sampleMsg, null, 2));
    } else {
      testsFailed++;
      log(colors.red, 'TEST', 'Message structure validation: FAILED');
    }

    // Test 6: Client disconnect handling
    console.log('\n--- Test 6: Client Disconnect ---');
    bob.ws.close();
    await wait(500);

    // Alice and Charlie should still be connected
    await sendMessage(alice, 'chat', 'Bob left the chat');
    await wait(500);

    if (charlie.messages.length > 0) {
      testsPassed++;
      log(colors.green, 'TEST', 'Disconnect handling: PASSED');
    } else {
      testsFailed++;
      log(colors.red, 'TEST', 'Disconnect handling: FAILED');
    }

    // Cleanup
    alice.ws.close();
    charlie.ws.close();
    await wait(500);

    // Summary
    console.log('\n' + '='.repeat(60));
    console.log('Test Summary');
    console.log('='.repeat(60));
    console.log(`${colors.green}Tests Passed: ${testsPassed}${colors.reset}`);
    console.log(`${colors.red}Tests Failed: ${testsFailed}${colors.reset}`);
    console.log(`Total: ${testsPassed + testsFailed}`);
    console.log('='.repeat(60) + '\n');

    process.exit(testsFailed > 0 ? 1 : 0);

  } catch (error) {
    log(colors.red, 'ERROR', `Test suite failed: ${error.message}`);
    console.error(error);
    process.exit(1);
  }
}

// Run tests
runTests();
