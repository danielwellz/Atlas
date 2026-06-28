# Atlas Unity Project (Unity as a Library)

This Unity project hosts Atlas anatomy/biomechanics scenes and exports Unity as a Library for React Native.

## Bridge contract

React Native native code sends messages to Unity with:
- GameObject: `AtlasBridge`
- Method: `OnReactNativeMessage(string json)`

Unity sends messages back to React Native with:
- Android: `com.atlasmobile.unity.AtlasUnityBridge.sendMessageToReact(topic, payload)`
- iOS: `AtlasUnitySendMessageToReact(topic, payload)`

RN -> Unity message envelope (transport) remains:

```json
{
  "topic": "anatomy.engine.v1",
  "payload": "{...command json...}"
}
```

Anatomy Engine command schema:
- version: `anatomy-engine.v1`
- topic: `anatomy.engine.v1`
- docs: `docs/anatomy-engine-schema-v1.md`

Legacy topic `anatomy.preview` is still accepted for backwards compatibility.

## Project setup

1. Open `atlas-unity` in Unity Hub.
2. Ensure your anatomy scene is added to Build Settings and enabled.
3. Keep/attach `AtlasBridgeBehaviour` in scene (or rely on runtime bootstrap script).
4. Add `BiomechanicsPlaybackController` to your anatomy scene root and wire:
   - `Animator` with states resolvable from animation key path candidates, in order:
     - `{exercise-folder}_{clip-name}` (for example `back_squat_clip_v1`)
     - `{exercise-folder}` (for example `back_squat`)
     - `{clip-name}` (for example `clip_v1`)
   - muscle renderer bindings per major muscle group.
   - optional `JointAngleOverlayController` for UI text/arc readouts.
5. For highlight materials, use shader `Atlas/MuscleHighlight` and expose:
   - `_HighlightColor`
   - `_HighlightIntensity`

## Export Android unityLibrary

From Unity Editor menu:
- `Atlas` -> `Export` -> `Android unityLibrary`

Expected output:
- `atlas-unity/Builds/android/unityLibrary`

`atlas-mobile/android/settings.gradle` automatically includes this module when that path exists.

## Export iOS Unity framework

From Unity Editor menu:
- `Atlas` -> `Export` -> `iOS Unity Export`

Then build `UnityFramework` from the exported Xcode project and copy the framework to:
- `atlas-unity/Builds/ios/UnityFramework.framework`

`atlas-mobile/ios/AtlasUnityBridge` pod automatically links this framework when the path exists.

## Notes

- Unity as a Library runs full-screen in Atlas MVP.
- A single Unity runtime instance is used by design.
- Mobile sends Anatomy Engine v1 commands from `/api/v1/exercises/{id}/biomechanics`.
