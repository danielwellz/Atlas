package com.atlasmobile.unity

import android.content.Intent
import com.facebook.react.bridge.LifecycleEventListener
import com.facebook.react.bridge.Promise
import com.facebook.react.bridge.ReactApplicationContext
import com.facebook.react.bridge.ReactContextBaseJavaModule
import com.facebook.react.bridge.ReactMethod

class UnityBridgeModule(
  private val reactApplicationContext: ReactApplicationContext,
) : ReactContextBaseJavaModule(reactApplicationContext), LifecycleEventListener {

  init {
    UnityBridgeRuntime.attachReactContext(reactApplicationContext)
    reactApplicationContext.addLifecycleEventListener(this)
  }

  override fun getName(): String = "UnityBridgeModule"

  @ReactMethod
  fun openUnity(promise: Promise) {
    val activity = currentActivity
    UnityBridgeRuntime.attachReactContext(reactApplicationContext)
    UnityBridgeRuntime.notifyUnityOpenRequested()

    if (activity == null) {
      UnityBridgeRuntime.notifyUnityOpenFailed(reason = "no_activity")
      promise.reject("E_NO_ACTIVITY", "openUnity requires an active React activity.")
      return
    }

    activity.runOnUiThread {
      try {
        val launchIntent =
          Intent(activity, UnityHostActivity::class.java).apply {
            addFlags(Intent.FLAG_ACTIVITY_REORDER_TO_FRONT)
          }

        activity.startActivity(launchIntent)
        promise.resolve(null)
      } catch (error: Throwable) {
        UnityBridgeRuntime.notifyUnityOpenFailed(reason = "launch_exception")
        promise.reject("E_OPEN_UNITY", "Failed to open Unity host activity.", error)
      }
    }
  }

  @ReactMethod
  fun closeUnity(promise: Promise) {
    UnityBridgeRuntime.closeUnityActivity()
    promise.resolve(null)
  }

  @ReactMethod
  fun sendMessageToUnity(topic: String, payload: String, promise: Promise) {
    try {
      UnityBridgeRuntime.sendMessageToUnity(topic = topic, payload = payload)
      promise.resolve(null)
    } catch (error: Throwable) {
      promise.reject("E_SEND_UNITY_MESSAGE", "Unable to send message to Unity.", error)
    }
  }

  @ReactMethod
  fun receiveMessageFromUnity(promise: Promise) {
    UnityBridgeRuntime.attachReactContext(reactApplicationContext)
    promise.resolve(true)
  }

  @ReactMethod
  fun addListener(eventName: String) {
    // Required for RN event emitter compatibility.
  }

  @ReactMethod
  fun removeListeners(count: Double) {
    // Required for RN event emitter compatibility.
  }

  override fun onHostResume() {
    UnityBridgeRuntime.attachReactContext(reactApplicationContext)
  }

  override fun onHostPause() {
    // No-op.
  }

  override fun onHostDestroy() {
    UnityBridgeRuntime.closeUnityActivity()
  }

  override fun invalidate() {
    reactApplicationContext.removeLifecycleEventListener(this)
    UnityBridgeRuntime.closeUnityActivity()
    super.invalidate()
  }
}
