#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

JBR="/Applications/Android Studio.app/Contents/jbr/Contents/Home"
if [[ -x "$JBR/bin/java" ]]; then
  export JAVA_HOME="$JBR"
  export PATH="$JAVA_HOME/bin:$PATH"
fi

DEFAULT_SDK="$HOME/Library/Android/sdk"
if [[ -d "$DEFAULT_SDK" ]]; then
  export ANDROID_HOME="${ANDROID_HOME:-$DEFAULT_SDK}"
  export ANDROID_SDK_ROOT="${ANDROID_SDK_ROOT:-$DEFAULT_SDK}"
fi

if [[ -d "android" ]]; then
  if [[ ! -f "android/local.properties" ]]; then
    SDK_DIR="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$DEFAULT_SDK}}"
    if [[ -d "$SDK_DIR" ]]; then
      mkdir -p android
      printf "sdk.dir=%s\n" "$SDK_DIR" > android/local.properties
    fi
  fi
fi

export NODE_ENV="${NODE_ENV:-development}"

exec npx expo run:android "$@"

