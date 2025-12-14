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
  
  // Initialize Spark Wallet silently
  
  // Use Spark SDK to get correct private key
  const { wallet } = await SparkWallet.initialize({
    mnemonicOrSeed: MNEMONIC,
    options: { 
      network: "MAINNET", 
      syncTokens: false,
      disableAutoSync: true
    },
  });

  // Get identityKey through signer from Spark SDK
  const signer = wallet.signingService?.config?.signer;
  const identityKey = signer?.identityKey;
  
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
  
  // Exit immediately after saving (to avoid waiting for SDK background processes)
  process.exit(0);
}

main().catch((err) => {
  logError(`Error: ${err.message}`, null);
  process.exit(1);
});
