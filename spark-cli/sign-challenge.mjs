import { SparkWallet } from "@buildonspark/spark-sdk";
import * as ecc from "tiny-secp256k1";
import { createHash } from "crypto";
import { readFileSync, writeFileSync } from "fs";
import { fileURLToPath } from "url";
import { dirname, join } from "path";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const MNEMONIC = "betray apple salon citizen source spot spice goose risk strike apple mesh";
const EXPECTED_PUBKEY = "038ad2deab88fa2f278ad895f61254a804370d987db61301a7d6872df4231b6597";

// ANSI colors for terminal
const colors = {
  reset: "\x1b[0m",
  green: "\x1b[32m",
  red: "\x1b[31m",
};

// Logger function that mimics Go zap format
function logSuccess(message, durationMs = null) {
  const time = new Date().toLocaleTimeString("en-US", { 
    hour12: false, 
    hour: "2-digit", 
    minute: "2-digit", 
    second: "2-digit" 
  });
  const durationStr = durationMs !== null ? ` (${durationMs}ms)` : "";
  console.log(`${time}        ${colors.green}SUCCESS${colors.reset} ✓ ${message}${durationStr}`);
}

function logError(message, durationMs = null) {
  const time = new Date().toLocaleTimeString("en-US", { 
    hour12: false, 
    hour: "2-digit", 
    minute: "2-digit", 
    second: "2-digit" 
  });
  const durationStr = durationMs !== null ? ` (${durationMs}ms)` : "";
  console.log(`${time}        ${colors.red}ERROR${colors.reset} ✗ ${message}${durationStr}`);
}

function encodeToDER(sig) {
  const r = sig.slice(0, 32);
  const s = sig.slice(32, 64);

  const trim = (b) => {
    let i = 0;
    while (i < b.length && b[i] === 0) i++;
    let res = b.slice(i);
    if (res[0] & 0x80) res = Buffer.concat([Buffer.from([0]), res]);
    return res;
  };

  const rEnc = trim(r);
  const sEnc = trim(s);

  return Buffer.concat([
    Buffer.from([0x30, rEnc.length + sEnc.length + 4]),
    Buffer.from([0x02, rEnc.length]),
    rEnc,
    Buffer.from([0x02, sEnc.length]),
    sEnc,
  ]);
}

// Handle unhandled promise rejections (from Spark SDK background processes)
process.on('unhandledRejection', (reason, promise) => {
  // Ignore Spark SDK sync errors as they're non-critical for signing
  if (reason && typeof reason === 'object' && reason.message) {
    if (reason.message.includes('request_leaves_swap') || 
        reason.message.includes('request_swap_leaves') ||
        reason.message.includes('Something went wrong')) {
      // These are Spark SDK sync errors - non-critical for signing challenge
      // Don't log them as errors, just continue
      return;
    }
  }
  // Log other unhandled rejections
  console.error('Unhandled Rejection at:', promise, 'reason:', reason);
});

// Handle uncaught exceptions
process.on('uncaughtException', (err) => {
  // Ignore Spark SDK sync errors
  if (err.message && (
    err.message.includes('request_leaves_swap') ||
    err.message.includes('request_swap_leaves') ||
    err.message.includes('Something went wrong')
  )) {
    // Non-critical sync error, don't crash
    return;
  }
  // Log other exceptions
  console.error('Uncaught Exception:', err);
});

async function main() {
  const startTime = Date.now();
  let challengeString = process.argv[2];
  
  // If challenge not provided, read from challenge.json
  if (!challengeString) {
    try {
      const challengePath = join(__dirname, "..", "data_in", "challenge.json");
      const challengeData = JSON.parse(readFileSync(challengePath, "utf8"));
      challengeString = challengeData.challengeString;
      // Challenge loaded silently
    } catch (err) {
      const duration = Date.now() - startTime;
      logError(`Failed to read challenge from file: ${err.message}`, duration);
      process.exit(1);
    }
  }
  
  if (!challengeString) {
    const duration = Date.now() - startTime;
    logError("Challenge not specified", duration);
    process.exit(1);
  }
  
  // Initialize Spark Wallet and get identityKey quickly
  // Strategy: Initialize SDK, get key immediately, exit before sync starts
  let identityKey = null;
  
  try {
    // Initialize Spark SDK
    // Note: Even with disableAutoSync, SDK may start background sync
    // We need to get the key quickly and exit before sync causes issues
    const initPromise = SparkWallet.initialize({
      mnemonicOrSeed: MNEMONIC,
      options: { 
        network: "MAINNET", 
        syncTokens: false,
        disableAutoSync: true
      },
    });
    
    // Use shorter timeout - we want to get key and exit quickly
    const timeoutPromise = new Promise((_, reject) => 
      setTimeout(() => reject(new Error("Initialization timeout")), 5000)
    );
    
    const result = await Promise.race([initPromise, timeoutPromise]);
    const wallet = result.wallet;
    
    // Get identityKey immediately after initialization
    // Do this before SDK starts background sync
    const signer = wallet?.signingService?.config?.signer;
    identityKey = signer?.identityKey;
    
    if (!identityKey) {
      throw new Error("identityKey not found in wallet");
    }
  } catch (err) {
    // If initialization fails, check if it's a sync error (non-critical)
    // Sync errors happen after initialization, so wallet might still be usable
    if (err.message && (
      err.message.includes("request_leaves_swap") ||
      err.message.includes("request_swap_leaves") ||
      err.message.includes("Something went wrong")
    )) {
      // Sync error occurred, but wallet might be initialized
      // Try to get identityKey from error object or retry quickly
      if (err.wallet) {
        const signer = err.wallet?.signingService?.config?.signer;
        identityKey = signer?.identityKey;
      }
      
      // If still no key, try one more quick initialization
      if (!identityKey) {
        try {
          const quickRetry = await Promise.race([
            SparkWallet.initialize({
              mnemonicOrSeed: MNEMONIC,
              options: { 
                network: "MAINNET", 
                syncTokens: false,
                disableAutoSync: true
              },
            }),
            new Promise((_, reject) => setTimeout(() => reject(new Error("Retry timeout")), 3000))
          ]);
          
          const quickSigner = quickRetry.wallet?.signingService?.config?.signer;
          identityKey = quickSigner?.identityKey;
        } catch (retryErr) {
          // Retry also failed
        }
      }
    }
    
    // If we still don't have identityKey, exit with error
    if (!identityKey) {
      const duration = Date.now() - startTime;
      logError(`Failed to get identityKey: ${err.message}`, duration);
      if (err.message.includes("timeout") || err.message.includes("request_leaves_swap")) {
        logError("Spark SDK sync is failing. This is a known issue.", duration);
        logError("Please try again in a few minutes.", duration);
      }
      process.exit(1);
    }
  }
  
  if (!identityKey) {
    const duration = Date.now() - startTime;
    logError("Failed to get identityKey from Spark Wallet", duration);
    process.exit(1);
  }
  
  // Extract private key from identityKey
  let privateKey = null;
  
  if (Buffer.isBuffer(identityKey) && identityKey.length === 32) {
    privateKey = identityKey;
  } else if (identityKey.privateKey) {
    privateKey = identityKey.privateKey;
  } else if (identityKey.key) {
    privateKey = identityKey.key;
  } else if (identityKey._key) {
    privateKey = identityKey._key;
  } else if (typeof identityKey === 'object') {
    for (const key of Object.getOwnPropertyNames(identityKey)) {
      const value = identityKey[key];
      if (Buffer.isBuffer(value) && value.length === 32) {
        privateKey = value;
        break;
      }
    }
  }
  
  if (!privateKey) {
    const duration = Date.now() - startTime;
    logError("Failed to extract private key from identityKey", duration);
    process.exit(1);
  }
  
  // Convert to Buffer if needed
  if (!Buffer.isBuffer(privateKey)) {
    if (privateKey instanceof Uint8Array) {
      privateKey = Buffer.from(privateKey);
    } else if (typeof privateKey === 'string') {
      privateKey = Buffer.from(privateKey, 'hex');
    }
  }
  
  // Verify public key for confirmation
  const pubKeyHex = Buffer.from(ecc.pointFromScalar(privateKey, true)).toString("hex");
  
  if (pubKeyHex !== EXPECTED_PUBKEY) {
    const duration = Date.now() - startTime;
    logError(`Public key mismatch. Expected: ${EXPECTED_PUBKEY}, Got: ${pubKeyHex}`, duration);
    process.exit(1);
  }
  
  // Sign challenge (silently)
  const msgHash = createHash("sha256").update(challengeString).digest();
  const signatureRaw = ecc.sign(msgHash, privateKey);
  const signatureDER = encodeToDER(signatureRaw);
  const signatureHex = signatureDER.toString("hex");
  
  // Automatically save signature to signature.json
  try {
    const signaturePath = join(__dirname, "..", "data_in", "signature.json");
    let signatureData = {};
    
    // Read existing file if exists
    try {
      const existingData = readFileSync(signaturePath, "utf8");
      signatureData = JSON.parse(existingData);
    } catch (err) {
      signatureData = {
        publicKey: "",
        signature: "",
        requestId: ""
      };
    }
    
    // Update signature
    signatureData.signature = signatureHex;
    
    // Always synchronize publicKey and requestId from challenge.json
    try {
      const challengePath = join(__dirname, "..", "data_in", "challenge.json");
      const challengeData = JSON.parse(readFileSync(challengePath, "utf8"));
      signatureData.publicKey = challengeData.publicKey || EXPECTED_PUBKEY;
      signatureData.requestId = challengeData.requestId || "";
    } catch (err) {
      if (!signatureData.publicKey) {
        signatureData.publicKey = EXPECTED_PUBKEY;
      }
    }
    
    // Save updated file
    writeFileSync(signaturePath, JSON.stringify(signatureData, null, 2), "utf8");
    const duration = Date.now() - startTime;
    logSuccess("Signature saved to signature.json", duration);
  } catch (err) {
    const duration = Date.now() - startTime;
    logError(`Failed to save signature: ${err.message}`, duration);
    process.exit(1);
  }
  
  // Exit immediately after saving (to avoid waiting for SDK background sync processes)
  // This prevents errors from Spark SDK sync (like request_leaves_swap) from affecting the signing process
  // The signature is already saved, so we can exit safely
  setTimeout(() => {
    process.exit(0);
  }, 100); // Small delay to ensure file write is complete
}

main().catch((err) => {
  // Check if error is related to Spark SDK sync (non-critical for signing)
  if (err.message && (
    err.message.includes("request_leaves_swap") ||
    err.message.includes("request_swap_leaves") ||
    err.message.includes("Something went wrong")
  )) {
    logError(`Spark SDK sync error (may be non-critical): ${err.message}`, null);
    logError("If signature was saved successfully, you can continue with verification.", null);
    // Don't exit with error code if it's just a sync issue
    // Check if signature file exists
    try {
      const signaturePath = join(__dirname, "..", "data_in", "signature.json");
      const signatureData = JSON.parse(readFileSync(signaturePath, "utf8"));
      if (signatureData.signature) {
        logError("Signature file exists - sync error may be non-critical", null);
        process.exit(0); // Exit successfully if signature was saved
      }
    } catch (fileErr) {
      // Signature file doesn't exist or is invalid
    }
  }
  logError(`Error: ${err.message}`, null);
  if (err.stack) {
    console.error(err.stack);
  }
  process.exit(1);
});
