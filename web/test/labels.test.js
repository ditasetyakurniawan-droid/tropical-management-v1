import assert from "node:assert/strict";
import test from "node:test";

import {
  channelLabel,
  roleLabel,
  severityLabel,
  statusLabel,
} from "../lib/labels.js";

test("label helpers translate known values", () => {
  assert.equal(roleLabel("admin"), "Admin");
  assert.equal(statusLabel("in_progress"), "Diproses");
  assert.equal(severityLabel("critical"), "Kritis");
  assert.equal(channelLabel("dine-in"), "Makan di Tempat");
});

test("label helpers preserve unknown values and provide fallbacks", () => {
  assert.equal(roleLabel("owner"), "owner");
  assert.equal(statusLabel("waiting_review"), "waiting review");
  assert.equal(severityLabel("urgent"), "urgent");
  assert.equal(channelLabel("kiosk"), "kiosk");
  assert.equal(roleLabel(""), "-");
  assert.equal(statusLabel(null), "-");
  assert.equal(severityLabel(undefined), "-");
  assert.equal(channelLabel(null), "-");
});
