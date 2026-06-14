/*
 * esp32_sensor.ino - Tier 1 IoT device firmware (reference implementation)
 *
 * Part of the four-tier blockchain-edge IoT security framework:
 *   "A Hardware-Validated Permissioned Blockchain-Edge Security Framework
 *    for Industrial IoT Smart Spaces"  (Section IV-B)
 *
 * Demonstrates:
 *   - AES-128-CBC encryption using the ESP32 hardware AES accelerator
 *     (0.09 ms/KB vs 1.30 ms/KB software -> 14.4x speedup, Section V-F)
 *   - Device DID embedded as associated data
 *   - MQTT-over-Wi-Fi (WPA2) transmission to the Tier 2 gateway
 *
 * Hardware: ESP32-WROOM-32 (dual-core Xtensa LX6 @ 240 MHz, 520 KB SRAM)
 * Libraries: WiFi.h, PubSubClient.h, mbedtls/aes.h (bundled with ESP-IDF)
 *
 * SPDX-License-Identifier: Apache-2.0
 */

#include <WiFi.h>
#include <PubSubClient.h>
#include "mbedtls/aes.h"

// ---- Configuration (replace with your testbed values) ----
const char* WIFI_SSID     = "SMARTSPACE_AP";
const char* WIFI_PASSWORD = "<wpa2-passphrase>";
const char* MQTT_BROKER   = "192.168.1.10";   // Tier 2 Raspberry Pi gateway
const int   MQTT_PORT     = 1883;
const char* DEVICE_DID    = "did:smartspace:esp32:0001";
const char* MQTT_TOPIC    = "smartspace/ingest";

// 128-bit pre-shared key (Zone 1). In production this is provisioned into
// eFuse at manufacture (Section IV-H); shown here as a constant for clarity.
static const uint8_t AES_KEY[16] = {
  0x2b,0x7e,0x15,0x16,0x28,0xae,0xd2,0xa6,
  0xab,0xf7,0x15,0x88,0x09,0xcf,0x4f,0x3c
};

WiFiClient   wifiClient;
PubSubClient mqtt(wifiClient);

// Encrypt a 16-byte-aligned buffer with the ESP32 hardware AES engine.
// Returns elapsed microseconds for benchmarking (Section V-F).
unsigned long aes128_cbc_encrypt_hw(const uint8_t* in, uint8_t* out,
                                    size_t len, uint8_t iv[16]) {
  mbedtls_aes_context ctx;
  mbedtls_aes_init(&ctx);
  mbedtls_aes_setkey_enc(&ctx, AES_KEY, 128);

  unsigned long t0 = micros();
  mbedtls_aes_crypt_cbc(&ctx, MBEDTLS_AES_ENCRYPT, len, iv,
                        (const unsigned char*)in, (unsigned char*)out);
  unsigned long elapsed = micros() - t0;

  mbedtls_aes_free(&ctx);
  return elapsed;
}

void connectWiFi() {
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  while (WiFi.status() != WL_CONNECTED) { delay(300); }
}

void connectMQTT() {
  mqtt.setServer(MQTT_BROKER, MQTT_PORT);
  while (!mqtt.connected()) {
    if (mqtt.connect(DEVICE_DID)) break;
    delay(500);
  }
}

void setup() {
  Serial.begin(115200);
  connectWiFi();
  connectMQTT();
}

void loop() {
  if (!mqtt.connected()) connectMQTT();
  mqtt.loop();

  // 1) Generate a sensor event (Step 1, Fig. 2).
  char payload[64];
  float reading = 20.0 + (float)(esp_random() % 1000) / 100.0;  // demo value
  int n = snprintf(payload, sizeof(payload), "{\"did\":\"%s\",\"v\":%.2f}",
                   DEVICE_DID, reading);

  // Pad to 16-byte AES block boundary (PKCS#7-style padding).
  size_t padded = ((n / 16) + 1) * 16;
  uint8_t in[80] = {0}, out[80] = {0};
  memcpy(in, payload, n);
  uint8_t pad = padded - n;
  for (size_t i = n; i < padded; i++) in[i] = pad;

  // 2) Encrypt with hardware AES-128-CBC (Step 1, Fig. 2; Zone 1).
  uint8_t iv[16];
  for (int i = 0; i < 16; i++) iv[i] = esp_random() & 0xFF;
  unsigned long enc_us = aes128_cbc_encrypt_hw(in, out, padded, iv);
  Serial.printf("AES-128 HW encrypt: %lu us for %u bytes\n", enc_us, padded);

  // 3) Transmit IV + ciphertext to the Tier 2 gateway over MQTT/WPA2
  //    (Step 2, Fig. 2; Device -> Edge).
  uint8_t frame[96];
  memcpy(frame, iv, 16);
  memcpy(frame + 16, out, padded);
  mqtt.publish(MQTT_TOPIC, frame, 16 + padded);

  delay(1000);  // 1 Hz reporting (matches the 1 tx/device/s workload)
}
