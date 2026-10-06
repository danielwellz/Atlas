This is a new [**React Native**](https://reactnative.dev) project, bootstrapped using [`@react-native-community/cli`](https://github.com/react-native-community/cli).

# Getting Started

> **Note**: Make sure you have completed the [Set Up Your Environment](https://reactnative.dev/docs/set-up-your-environment) guide before proceeding.

## Step 1: Start Metro

First, you will need to run **Metro**, the JavaScript build tool for React Native.

To start the Metro dev server, run the following command from the root of your React Native project:

```sh
# Using npm
npm start

# OR using Yarn
yarn start
```

## Step 2: Build and run your app

With Metro running, open a new terminal window/pane from the root of your React Native project, and use one of the following commands to build and run your Android or iOS app:

### Android

```sh
# Using npm
npm run android

# OR using Yarn
yarn android
```

### iOS

For iOS, remember to install CocoaPods dependencies (this only needs to be run on first clone or after updating native deps).

The first time you create a new project, run the Ruby bundler to install CocoaPods itself:

```sh
bundle install
```

Then, and every time you update your native dependencies, run:

```sh
bundle exec pod install
```

For more information, please visit [CocoaPods Getting Started guide](https://guides.cocoapods.org/using/getting-started.html).

```sh
# Using npm
npm run ios

# OR using Yarn
yarn ios
```

If everything is set up correctly, you should see your new app running in the Android Emulator, iOS Simulator, or your connected device.

This is one way to run your app — you can also build it directly from Android Studio or Xcode.

## Environments (API base URL)

The JS bundle is built for one environment, chosen at build time. There is no runtime switch.

| Environment | API base URL | Selected by |
|---|---|---|
| `local` | `http://10.0.2.2:8080` (Android emulator), `http://localhost:8080` (iOS simulator) | Android `debug` / `debugOptimized`, iOS `Debug` (default) |
| `staging` | `TODO(owner)`, HTTPS only | Android `staging` build type, iOS `Staging` configuration (not created yet, see T18) |
| `prod` | `TODO(owner)`, HTTPS only | Android `release`, iOS `Release` |

How it works:

- `src/config/environments.json` holds the per-environment defaults. Owners: replace the `TODO(owner)` staging and prod URLs there, or supply them at build time with `ATLAS_API_BASE_URL` (for example from a CI secret).
- `scripts/generate-env.js` writes `src/config/env.generated.ts` (gitignored), which records the environment and any URL override. It runs on `npm ci`/`npm install` (only if the file is missing), before `npm test` and `npm run typecheck` (same), and on every native build: Gradle runs `generateAtlasEnv<Variant>` and Xcode runs `scripts/xcode-generate-env.sh` from the "Bundle React Native code and images" phase.
- The script fails the build if the URL is still a placeholder, is not a valid URL, or is not `https://` for staging or prod.
- At startup `src/config` resolves the config (`appConfig.apiBaseUrl`) and throws if the URL is a placeholder, if staging or prod is not HTTPS, or if a release build (`__DEV__ === false`) would use plain HTTP. `src/api/client.ts` takes its base URL from there.

Common tasks:

```sh
# Debug build against a backend on your LAN (physical device)
ATLAS_API_BASE_URL=http://192.168.1.20:8080 npm run env
npm start

# Debug build against staging (debug builds honour ATLAS_ENV; staging/release ignore it)
ATLAS_ENV=staging ATLAS_API_BASE_URL=https://api.staging.example.com npm run env
ATLAS_ENV=staging ATLAS_API_BASE_URL=https://api.staging.example.com npm run android

# Staging and production APKs
cd android
ATLAS_API_BASE_URL=https://api.staging.example.com ./gradlew assembleStaging
ATLAS_API_BASE_URL=https://api.example.com ./gradlew assembleRelease

# Back to local defaults
npm run env
```

Notes:

- Debug builds always regenerate the file on build, so a debug build with `ATLAS_ENV` unset goes back to `local`. Metro serves whatever `env.generated.ts` currently says, so rerun `npm run env` (or a debug build) after building staging or release locally.
- All variants share one generated file, so build one non-debug variant per Gradle invocation (`assembleStaging` and `assembleRelease` separately, not together).
- Android `staging` is a copy of `release` (bundled JS, no cleartext traffic, same signing for now; release signing is T17). It uses the same application ID as release, so it replaces a release install on the same device.
- iOS: the `Staging` Xcode configuration is not created yet (deferred with T18). Until then, iOS staging builds need a `Staging` configuration added in Xcode; `scripts/xcode-generate-env.sh` already maps it.

## Unity As A Library (Anatomy)

Atlas now includes an `Anatomy` tab that opens Unity full-screen through a native bridge module named `UnityBridgeModule`.

- Unity project lives at `/Users/danielwellz/Work/Atlas/atlas-unity`.
- Android auto-links `unityLibrary` when this path exists:
  `atlas-unity/Builds/android/unityLibrary`
- iOS auto-links `UnityFramework.framework` through the local `AtlasUnityBridge` pod when this path exists:
  `atlas-unity/Builds/ios/UnityFramework.framework`

Unity bridge object/method contract:

- GameObject: `AtlasBridge`
- Method: `OnReactNativeMessage(string json)`

React Native API:

- `openUnity()`
- `closeUnity()`
- `sendMessageToUnity(topic, payload)`
- `receiveMessageFromUnity(callback)`
- `receiveUnityState(callback)` for `loading/loaded/failed/closed`

Anatomy Engine v1 helper API (`src/native/anatomyEngineBridge.ts`):

- `loadExerciseBiomechanics(...)`
- `setHighlightMuscles(...)`
- `setLayerVisibility(...)`
- `setJointAngleOverlay(...)`

Schema reference: `../docs/anatomy-engine-schema-v1.md`

## Form Check v1 (On-Device + Opt-In Upload)

Atlas now includes a `Form Check` screen for squat form analysis.

- Native module: `FormCheckPoseModule` (Android + iOS bridge with frame streaming API)
- JS wrapper: `src/native/formCheckPose.ts`
- Scoring model: `src/features/formCheck/scoring.ts`
- Screen: `src/screens/workout/FormCheckScreen.tsx`

Privacy and consent enforcement:

- Local-only by default, gated by `form_check_local` consent.
- Upload is optional and requires:
  - `form_check_upload` entitlement
  - `form_check_upload` consent
  - explicit user action (`Upload to Coach`)
- No background upload behavior is implemented.

Policy reference: `../docs/form-check-privacy-v1.md`

## Step 3: Modify your app

Now that you have successfully run the app, let's make changes!

Open `App.tsx` in your text editor of choice and make some changes. When you save, your app will automatically update and reflect these changes — this is powered by [Fast Refresh](https://reactnative.dev/docs/fast-refresh).

When you want to forcefully reload, for example to reset the state of your app, you can perform a full reload:

- **Android**: Press the <kbd>R</kbd> key twice or select **"Reload"** from the **Dev Menu**, accessed via <kbd>Ctrl</kbd> + <kbd>M</kbd> (Windows/Linux) or <kbd>Cmd ⌘</kbd> + <kbd>M</kbd> (macOS).
- **iOS**: Press <kbd>R</kbd> in iOS Simulator.

## Congratulations! :tada:

You've successfully run and modified your React Native App. :partying_face:

### Now what?

- If you want to add this new React Native code to an existing application, check out the [Integration guide](https://reactnative.dev/docs/integration-with-existing-apps).
- If you're curious to learn more about React Native, check out the [docs](https://reactnative.dev/docs/getting-started).

# Troubleshooting

If you're having issues getting the above steps to work, see the [Troubleshooting](https://reactnative.dev/docs/troubleshooting) page.

# Learn More

To learn more about React Native, take a look at the following resources:

- [React Native Website](https://reactnative.dev) - learn more about React Native.
- [Getting Started](https://reactnative.dev/docs/environment-setup) - an **overview** of React Native and how setup your environment.
- [Learn the Basics](https://reactnative.dev/docs/getting-started) - a **guided tour** of the React Native **basics**.
- [Blog](https://reactnative.dev/blog) - read the latest official React Native **Blog** posts.
- [`@facebook/react-native`](https://github.com/facebook/react-native) - the Open Source; GitHub **repository** for React Native.
