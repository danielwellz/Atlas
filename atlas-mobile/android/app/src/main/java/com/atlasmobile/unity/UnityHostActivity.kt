package com.atlasmobile.unity

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.graphics.Color
import android.os.Bundle
import android.view.Gravity
import android.view.View
import android.widget.Button
import android.widget.FrameLayout
import android.widget.TextView
import java.lang.reflect.Method

class UnityHostActivity : Activity() {
  private var unityPlayer: Any? = null
  private var unityPlayerClass: Class<*>? = null
  private var launchMode: String = "fallback"

  override fun onCreate(savedInstanceState: Bundle?) {
    super.onCreate(savedInstanceState)

    launchMode = if (attachUnityPlayer()) "unity" else "fallback"
    UnityBridgeRuntime.onUnityActivityCreated(activity = this, mode = launchMode)
  }

  override fun onNewIntent(intent: Intent) {
    super.onNewIntent(intent)
    invokeUnityMethod("newIntent", Intent::class.java, intent)
  }

  override fun onResume() {
    super.onResume()
    invokeUnityMethod("resume")
  }

  override fun onPause() {
    invokeUnityMethod("pause")
    super.onPause()
  }

  override fun onWindowFocusChanged(hasFocus: Boolean) {
    super.onWindowFocusChanged(hasFocus)
    invokeUnityMethod("windowFocusChanged", Boolean::class.javaPrimitiveType, hasFocus)
  }

  override fun onDestroy() {
    invokeUnityMethod("destroy")
    unityPlayer = null
    unityPlayerClass = null
    UnityBridgeRuntime.onUnityActivityDestroyed(activity = this, mode = launchMode)
    super.onDestroy()
  }

  private fun attachUnityPlayer(): Boolean {
    return try {
      val playerClass = Class.forName("com.unity3d.player.UnityPlayer")
      val constructor = playerClass.getConstructor(Context::class.java)
      val playerInstance = constructor.newInstance(this)

      unityPlayerClass = playerClass
      unityPlayer = playerInstance
      setContentView(playerInstance as View)
      invokeUnityMethod("requestFocus")
      true
    } catch (_: Throwable) {
      setContentView(createFallbackView())
      false
    }
  }

  private fun createFallbackView(): View {
    val root =
      FrameLayout(this).apply {
        setBackgroundColor(Color.parseColor("#020617"))
      }

    val title =
      TextView(this).apply {
        text = "Unity library is not packaged yet."
        setTextColor(Color.WHITE)
        textSize = 18f
        gravity = Gravity.CENTER
      }

    val subtitle =
      TextView(this).apply {
        text = "Export atlas-unity as Unity Library to enable full runtime."
        setTextColor(Color.parseColor("#94A3B8"))
        textSize = 14f
        gravity = Gravity.CENTER
      }

    val closeButton =
      Button(this).apply {
        text = "Close"
        setOnClickListener { finish() }
      }

    val titleLayoutParams =
      FrameLayout.LayoutParams(
        FrameLayout.LayoutParams.MATCH_PARENT,
        FrameLayout.LayoutParams.WRAP_CONTENT,
      ).apply {
        gravity = Gravity.CENTER_HORIZONTAL or Gravity.CENTER_VERTICAL
        marginStart = dp(24)
        marginEnd = dp(24)
        topMargin = -dp(28)
      }

    val subtitleLayoutParams =
      FrameLayout.LayoutParams(
        FrameLayout.LayoutParams.MATCH_PARENT,
        FrameLayout.LayoutParams.WRAP_CONTENT,
      ).apply {
        gravity = Gravity.CENTER_HORIZONTAL or Gravity.CENTER_VERTICAL
        marginStart = dp(24)
        marginEnd = dp(24)
        topMargin = dp(12)
      }

    val closeLayoutParams =
      FrameLayout.LayoutParams(
        FrameLayout.LayoutParams.WRAP_CONTENT,
        FrameLayout.LayoutParams.WRAP_CONTENT,
        Gravity.BOTTOM or Gravity.CENTER_HORIZONTAL,
      ).apply {
        bottomMargin = dp(40)
      }

    root.addView(title, titleLayoutParams)
    root.addView(subtitle, subtitleLayoutParams)
    root.addView(closeButton, closeLayoutParams)

    return root
  }

  private fun dp(value: Int): Int {
    val density = resources.displayMetrics.density
    return (value * density).toInt()
  }

  private fun invokeUnityMethod(name: String, parameterType: Class<*>? = null, argument: Any? = null) {
    val player = unityPlayer ?: return
    val playerClass = unityPlayerClass ?: return

    val method =
      try {
        if (parameterType == null) {
          playerClass.getMethod(name)
        } else {
          playerClass.getMethod(name, parameterType)
        }
      } catch (_: Throwable) {
        null
      }

    method?.invokeCatching(player, argument)
  }

  private fun Method.invokeCatching(instance: Any, argument: Any?) {
    try {
      if (parameterTypes.isEmpty()) {
        invoke(instance)
      } else {
        invoke(instance, argument)
      }
    } catch (_: Throwable) {
      // Ignore lifecycle invocation failures while Unity runtime is unavailable.
    }
  }
}
