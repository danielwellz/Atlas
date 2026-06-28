package com.atlasmobile.unity

import android.util.Log
import com.facebook.react.bridge.Arguments
import com.facebook.react.bridge.ReactApplicationContext
import com.facebook.react.modules.core.DeviceEventManagerModule
import java.lang.ref.WeakReference
import java.util.ArrayDeque
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference
import org.json.JSONObject

object UnityBridgeRuntime {
  private const val TAG = "UnityBridgeRuntime"
  private const val UNITY_EVENT_NAME = "UnityMessage"
  private const val UNITY_LIFECYCLE_TOPIC = "unity.lifecycle"
  private const val UNITY_STATE_TOPIC = "unity.state"
  private const val UNITY_GAME_OBJECT = "AtlasBridge"
  private const val UNITY_RECEIVER_METHOD = "OnReactNativeMessage"
  private const val MAX_PENDING_MESSAGES = 64

  private val reactContextRef = AtomicReference<WeakReference<ReactApplicationContext>?>(null)
  private val unityActivityRef = AtomicReference<WeakReference<UnityHostActivity>?>(null)
  private val openRequested = AtomicBoolean(false)
  private val openCount = AtomicInteger(0)
  private val closeCount = AtomicInteger(0)
  private val pendingMessages = ArrayDeque<String>(MAX_PENDING_MESSAGES)

  fun attachReactContext(reactContext: ReactApplicationContext) {
    reactContextRef.set(WeakReference(reactContext))
  }

  fun onUnityActivityCreated(activity: UnityHostActivity, mode: String) {
    openRequested.set(false)
    unityActivityRef.set(WeakReference(activity))
    val currentOpenCount = openCount.incrementAndGet()
    emitLifecycleEvent(state = "opened", mode = mode, opens = currentOpenCount, closes = closeCount.get())
    if (mode == "unity") {
      emitStateEvent(state = "loaded", mode = mode, reason = "")
      drainPendingMessages()
    } else {
      emitStateEvent(state = "failed", mode = mode, reason = "unity_runtime_unavailable")
      clearPendingMessages()
    }
  }

  fun onUnityActivityDestroyed(activity: UnityHostActivity, mode: String) {
    openRequested.set(false)
    val currentActivity = unityActivityRef.get()?.get()
    if (currentActivity === activity) {
      unityActivityRef.set(null)
    }

    val currentCloseCount = closeCount.incrementAndGet()
    emitLifecycleEvent(state = "closed", mode = mode, opens = openCount.get(), closes = currentCloseCount)
    emitStateEvent(state = "closed", mode = mode, reason = "")
  }

  fun closeUnityActivity() {
    openRequested.set(false)
    clearPendingMessages()
    val unityActivity = unityActivityRef.get()?.get()
    if (unityActivity == null) {
      emitStateEvent(state = "closed", mode = "native", reason = "")
      return
    }

    unityActivity.runOnUiThread {
      it.finish()
    }
  }

  fun notifyUnityOpenRequested() {
    openRequested.set(true)
    emitStateEvent(state = "loading", mode = "native", reason = "")
  }

  fun notifyUnityOpenFailed(reason: String) {
    openRequested.set(false)
    emitStateEvent(state = "failed", mode = "native", reason = reason)
  }

  fun sendMessageToUnity(topic: String, payload: String): Boolean {
    val encodedMessage = encodeMessage(topic = topic, payload = payload)
    val delivered = trySendMessageToUnityRuntime(encodedMessage)

    if (!delivered) {
      val shouldQueue = openRequested.get() || unityActivityRef.get()?.get() != null
      if (shouldQueue) {
        enqueuePendingMessage(encodedMessage)
      } else {
        // Fallback keeps RN bridge testable even before unityLibrary export is dropped in.
        emitMessageToReact(topic = "unity.echo", payload = encodedMessage)
      }
    }

    return delivered
  }

  @JvmStatic
  fun receiveMessageFromUnity(topic: String?, payload: String?) {
    val normalizedTopic = topic?.takeIf { it.isNotBlank() } ?: "unity.message"
    emitMessageToReact(topic = normalizedTopic, payload = payload ?: "")
    if (normalizedTopic == "unity.ready") {
      emitStateEvent(state = "loaded", mode = "unity", reason = "")
      drainPendingMessages()
    }
  }

  private fun trySendMessageToUnityRuntime(encodedMessage: String): Boolean {
    return try {
      val unityPlayerClass = Class.forName("com.unity3d.player.UnityPlayer")
      val sendMethod =
        unityPlayerClass.getMethod(
          "UnitySendMessage",
          String::class.java,
          String::class.java,
          String::class.java,
        )

      sendMethod.invoke(null, UNITY_GAME_OBJECT, UNITY_RECEIVER_METHOD, encodedMessage)
      true
    } catch (error: Throwable) {
      Log.w(TAG, "Unity runtime unavailable; message routed to fallback bridge.", error)
      false
    }
  }

  private fun emitLifecycleEvent(state: String, mode: String, opens: Int, closes: Int) {
    val payload =
      JSONObject()
        .put("state", state)
        .put("mode", mode)
        .put("openCount", opens)
        .put("closeCount", closes)
        .toString()

    emitMessageToReact(topic = UNITY_LIFECYCLE_TOPIC, payload = payload)
  }

  private fun emitStateEvent(state: String, mode: String, reason: String) {
    val payload =
      JSONObject()
        .put("state", state)
        .put("mode", mode)
        .put("reason", reason)
        .put("openCount", openCount.get())
        .put("closeCount", closeCount.get())
        .toString()

    emitMessageToReact(topic = UNITY_STATE_TOPIC, payload = payload)
  }

  private fun emitMessageToReact(topic: String, payload: String) {
    val reactContext = reactContextRef.get()?.get() ?: return
    if (!reactContext.hasActiveReactInstance()) {
      return
    }

    val eventPayload = Arguments.createMap().apply {
      putString("topic", topic)
      putString("payload", payload)
    }

    reactContext
      .getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter::class.java)
      .emit(UNITY_EVENT_NAME, eventPayload)
  }

  private fun encodeMessage(topic: String, payload: String): String {
    return JSONObject()
      .put("topic", topic)
      .put("payload", payload)
      .toString()
  }

  @Synchronized
  private fun enqueuePendingMessage(encodedMessage: String) {
    if (pendingMessages.size >= MAX_PENDING_MESSAGES) {
      pendingMessages.removeFirst()
    }

    pendingMessages.addLast(encodedMessage)
  }

  private fun drainPendingMessages() {
    while (true) {
      val message = pollPendingMessage() ?: break
      if (!trySendMessageToUnityRuntime(message)) {
        enqueuePendingMessage(message)
        return
      }
    }
  }

  @Synchronized
  private fun pollPendingMessage(): String? {
    return if (pendingMessages.isEmpty()) null else pendingMessages.removeFirst()
  }

  @Synchronized
  private fun clearPendingMessages() {
    pendingMessages.clear()
  }
}
