import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// ── Custom Metrics ────────────────────────────────────────────
const paymentSuccessRate = new Rate('payment_success_rate');
const paymentDuration    = new Trend('payment_duration_ms', true);
const failedPayments     = new Counter('failed_payments');

// ── Load Profile ──────────────────────────────────────────────
export const options = {
  scenarios: {
    // Ramp up to steady state
    ramp_up: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 50  },  // Ramp up
        { duration: '3m', target: 100 },  // Steady state
        { duration: '1m', target: 200 },  // Peak load
        { duration: '2m', target: 200 },  // Sustained peak
        { duration: '1m', target: 0   },  // Ramp down
      ],
    },
    // Spike test
    spike: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 500 },  // Sudden spike
        { duration: '1m',  target: 500 },
        { duration: '30s', target: 0   },
      ],
      startTime: '8m',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<2000', 'p(99)<5000'],
    http_req_failed:   ['rate<0.01'],           // <1% error rate
    payment_success_rate: ['rate>0.99'],        // >99% success
    payment_duration_ms:  ['p(95)<1500'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY  = __ENV.API_KEY  || 'test_api_key_here';

const HEADERS = {
  'Content-Type':  'application/json',
  'Authorization': `Bearer ${API_KEY}`,
};

// ── Test Scenarios ────────────────────────────────────────────

export default function () {
  const scenario = Math.random();

  if (scenario < 0.5) {
    testQRCodePayment();
  } else if (scenario < 0.75) {
    testCreditCardPayment();
  } else if (scenario < 0.9) {
    testPromptPayPayment();
  } else {
    testGetPayment();
  }

  sleep(Math.random() * 2 + 0.5);
}

function testQRCodePayment() {
  const orderID = `order_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
  const start = Date.now();

  const res = http.post(`${BASE_URL}/v1/payments`, JSON.stringify({
    order_id:       orderID,
    amount:         Math.floor(Math.random() * 9900 + 100), // 100-10000 THB
    currency:       'THB',
    method:         'QR_CODE',
    provider:       'OMISE',
    description:    'Load test QR payment',
    customer_email: 'test@example.com',
    return_url:     'https://example.com/return',
    callback_url:   'https://example.com/webhook',
  }), { headers: HEADERS });

  const duration = Date.now() - start;
  paymentDuration.add(duration);

  const success = check(res, {
    'status is 201':           (r) => r.status === 201,
    'has payment id':          (r) => r.json('data.payment.id') !== null,
    'has qr_code_image':       (r) => r.json('data.qr_code_image') !== '',
    'payment status is valid': (r) => ['PENDING','SUCCESS'].includes(r.json('data.payment.status')),
    'response time < 3s':      (r) => r.timings.duration < 3000,
  });

  paymentSuccessRate.add(success);
  if (!success) {
    failedPayments.add(1);
    console.error(`QR Payment failed: ${res.status} - ${res.body}`);
  }
}

function testCreditCardPayment() {
  const orderID = `order_cc_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
  const start = Date.now();

  const res = http.post(`${BASE_URL}/v1/payments`, JSON.stringify({
    order_id:       orderID,
    amount:         Math.floor(Math.random() * 49900 + 100),
    currency:       'THB',
    method:         'CREDIT_CARD',
    provider:       'OMISE',
    description:    'Load test card payment',
    card_token:     'tokn_test_5oliykbj3he5rlr0r0h', // Omise test token
    customer_email: `test+${Date.now()}@example.com`,
  }), { headers: HEADERS });

  const duration = Date.now() - start;
  paymentDuration.add(duration);

  const success = check(res, {
    'status is 201 or 402':  (r) => [201, 402].includes(r.status),
    'has payment id':        (r) => r.json('data.payment.id') !== null || r.json('error.code') !== null,
    'response time < 3s':    (r) => r.timings.duration < 3000,
  });

  paymentSuccessRate.add(res.status === 201);
}

function testPromptPayPayment() {
  const orderID = `order_pp_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;

  const res = http.post(`${BASE_URL}/v1/payments`, JSON.stringify({
    order_id:     orderID,
    amount:       Math.floor(Math.random() * 4900 + 100),
    currency:     'THB',
    method:       'PROMPTPAY',
    provider:     'GBPRIMEPAY',
    description:  'Load test PromptPay',
    promptpay_id: '0891234567',
  }), { headers: HEADERS });

  check(res, {
    'status is 201': (r) => r.status === 201,
    'has qr data':   (r) => r.json('data.payment.qr_code_data') !== null,
  });
}

function testGetPayment() {
  // Use a known payment ID or skip
  const res = http.get(`${BASE_URL}/v1/payments?page=1&page_size=10`, { headers: HEADERS });
  check(res, {
    'list status is 200': (r) => r.status === 200,
    'has payments array': (r) => Array.isArray(r.json('data.payments')),
  });
}

// ── Setup/Teardown ────────────────────────────────────────────
export function setup() {
  console.log(`🚀 Load test starting against: ${BASE_URL}`);
  const res = http.get(`${BASE_URL}/health`);
  if (res.status !== 200) {
    throw new Error(`API is not healthy: ${res.status}`);
  }
  console.log('✅ API health check passed');
}

export function teardown(data) {
  console.log('✅ Load test completed');
}
