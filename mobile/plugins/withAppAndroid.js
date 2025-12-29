const { withAppBuildGradle, withGradleProperties, createRunOncePlugin } = require("@expo/config-plugins");

function parseXmxMb(jvmargs) {
  const m = String(jvmargs || "").match(/-Xmx(\d+)([kKmMgG])/);
  if (!m) return null;
  const n = Number(m[1]);
  if (!Number.isFinite(n) || n <= 0) return null;
  const unit = String(m[2] || "").toLowerCase();
  if (unit === "g") return n * 1024;
  if (unit === "m") return n;
  if (unit === "k") return n / 1024;
  return null;
}

function ensureGradleJvmArgs(value) {
  const desiredXmxMb = 4096;
  const desiredMetaspaceMb = 1024;

  let out = String(value || "").trim();
  if (!out) out = "-Xmx2048m -XX:MaxMetaspaceSize=512m";

  const currentXmx = parseXmxMb(out);
  if (currentXmx == null || currentXmx < desiredXmxMb) {
    out = out.replace(/-Xmx\d+[kKmMgG]/, "").trim();
    out = `${out} -Xmx${desiredXmxMb}m`.trim();
  }

  if (out.match(/MaxMetaspaceSize=/)) {
    out = out.replace(/-XX:MaxMetaspaceSize=\d+[kKmMgG]/, `-XX:MaxMetaspaceSize=${desiredMetaspaceMb}m`);
  } else {
    out = `${out} -XX:MaxMetaspaceSize=${desiredMetaspaceMb}m`.trim();
  }

  if (!out.includes("-Dfile.encoding=")) {
    out = `${out} -Dfile.encoding=UTF-8`.trim();
  }

  return out.replace(/\s+/g, " ");
}

function ensureMissingDimensionStrategy(appBuildGradle, dimension, flavor) {
  const contents = String(appBuildGradle || "");
  if (
    contents.includes(`missingDimensionStrategy '${dimension}', '${flavor}'`) ||
    contents.includes(`missingDimensionStrategy \"${dimension}\", \"${flavor}\"`)
  ) {
    return contents;
  }

  const re = /(\s*)defaultConfig\s*\{\s*\n([\s\S]*?)\n\1\}/m;
  if (!re.test(contents)) return contents;

  return contents.replace(re, (full, indent, inner) => {
    if (String(inner).includes("missingDimensionStrategy")) return full;
    return `${indent}defaultConfig {\n${inner}\n${indent}    missingDimensionStrategy '${dimension}', '${flavor}'\n${indent}}`;
  });
}

function ensureReleaseSigningConfig(appBuildGradle) {
  const contents = String(appBuildGradle || "");
  if (!contents.includes("signingConfigs")) return contents;

  const re = /(\s*)signingConfigs\s*\{\s*\n([\s\S]*?)\n\1\}/m;
  const match = contents.match(re);
  if (!match) return contents;
  const indent = match[1] || "";
  const inner = match[2] || "";
  if (String(inner).includes("release {")) return contents;

  return contents.replace(re, (_full, indent, inner) => {
    const releaseBlock = [
      `${indent}    release {`,
      `${indent}        def storeFilePath = System.getenv("APP_UPLOAD_STORE_FILE") ?: (project.hasProperty('APP_UPLOAD_STORE_FILE') ? APP_UPLOAD_STORE_FILE : null)`,
      `${indent}        if (storeFilePath) {`,
      `${indent}            storeFile file(storeFilePath)`,
      `${indent}            storePassword System.getenv("APP_UPLOAD_STORE_PASSWORD") ?: (project.hasProperty('APP_UPLOAD_STORE_PASSWORD') ? APP_UPLOAD_STORE_PASSWORD : "")`,
      `${indent}            keyAlias System.getenv("APP_UPLOAD_KEY_ALIAS") ?: (project.hasProperty('APP_UPLOAD_KEY_ALIAS') ? APP_UPLOAD_KEY_ALIAS : "")`,
      `${indent}            keyPassword System.getenv("APP_UPLOAD_KEY_PASSWORD") ?: (project.hasProperty('APP_UPLOAD_KEY_PASSWORD') ? APP_UPLOAD_KEY_PASSWORD : "")`,
      `${indent}        }`,
      `${indent}    }`,
    ].join("\n");
    return `${indent}signingConfigs {\n${inner}\n${releaseBlock}\n${indent}}`;
  });
}

function ensureReleaseBuildTypeSigning(appBuildGradle) {
  let contents = String(appBuildGradle || "");
  if (!contents.includes("buildTypes")) return contents;
  if (contents.includes("signingConfig signingConfigs.release") && contents.includes("signingConfigs.release.storeFile")) {
    return contents;
  }

  const re = /(buildTypes\s*\{[\s\S]*?release\s*\{[\s\S]*?\n)(\s*)signingConfig\s+signingConfigs\.debug/;
  if (!re.test(contents)) return contents;

  return contents.replace(re, (_full, prefix, indent) => {
    const snippet = [
      `${indent}signingConfig signingConfigs.release`,
      `${indent}if (signingConfigs.release.storeFile == null) {`,
      `${indent}    signingConfig signingConfigs.debug`,
      `${indent}}`,
    ].join("\n");
    return `${prefix}${snippet}`;
  });
}

function withAppAndroid(config) {
  config = withAppBuildGradle(config, (config) => {
    let contents = config.modResults.contents;
    contents = ensureMissingDimensionStrategy(contents, "store", "play");
    contents = ensureReleaseSigningConfig(contents);
    contents = ensureReleaseBuildTypeSigning(contents);
    config.modResults.contents = contents;
    return config;
  });

  config = withGradleProperties(config, (config) => {
    const props = config.modResults;
    const upsert = (key, value) => {
      const idx = props.findIndex((p) => p.type === "property" && p.key === key);
      if (idx >= 0) {
        props[idx].value = value;
      } else {
        props.push({ type: "property", key, value });
      }
    };

    upsert("org.gradle.jvmargs", ensureGradleJvmArgs(props.find((p) => p.type === "property" && p.key === "org.gradle.jvmargs")?.value));
    // Play Console now requires targetSdkVersion 35 (Android 15).
    upsert("android.compileSdkVersion", "35");
    upsert("android.targetSdkVersion", "35");
    upsert("android.buildToolsVersion", "35.0.0");
    // Avoid noisy warning when AGP hasn't been officially tested with this compileSdk.
    upsert("android.suppressUnsupportedCompileSdk", "35");
    config.modResults = props;
    return config;
  });

  return config;
}

module.exports = createRunOncePlugin(withAppAndroid, "withAppAndroid", "1.0.0");
