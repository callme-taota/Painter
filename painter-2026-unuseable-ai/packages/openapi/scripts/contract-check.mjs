import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const specPath = resolve(process.cwd(), "openapi.yaml");
const content = readFileSync(specPath, "utf8");
const legacySpecPath = resolve(process.cwd(), "openapi-legacy-compat.yaml");
const legacyContent = readFileSync(legacySpecPath, "utf8");

const requiredPaths = [
  "/identity/auth/login",
  "/content/articles",
  "/system/configs",
  "/analytics/overview",
];

const missing = requiredPaths.filter((p) => !content.includes(p));
if (missing.length > 0) {
  console.error("Missing required contract paths:", missing.join(", "));
  process.exit(1);
}

const requiredLegacyPaths = ["/user/login/email", "/article/create", "/common/entry"];
const missingLegacy = requiredLegacyPaths.filter((p) => !legacyContent.includes(p));
if (missingLegacy.length > 0) {
  console.error("Missing legacy compatibility paths:", missingLegacy.join(", "));
  process.exit(1);
}

console.log("OpenAPI contract check passed.");
