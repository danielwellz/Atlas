package com.atlasmobile.formcheck

import android.os.Handler
import android.os.Looper
import com.facebook.react.bridge.Arguments
import com.facebook.react.bridge.LifecycleEventListener
import com.facebook.react.bridge.Promise
import com.facebook.react.bridge.ReactApplicationContext
import com.facebook.react.bridge.ReactContextBaseJavaModule
import com.facebook.react.bridge.ReactMethod
import com.facebook.react.modules.core.DeviceEventManagerModule
import kotlin.math.abs
import kotlin.math.max
import kotlin.math.min
import kotlin.math.roundToInt
import kotlin.math.sin

class FormCheckPoseModule(
  private val reactContext: ReactApplicationContext,
) : ReactContextBaseJavaModule(reactContext), LifecycleEventListener {

  companion object {
    private const val FRAME_EVENT_NAME = "FormCheckPoseFrame"
    private const val FRAME_INTERVAL_MS = 120L
  }

  private data class AngleFrame(
    val timestampMs: Long,
    val leftKneeDeg: Double,
    val rightKneeDeg: Double,
    val leftHipDeg: Double,
    val rightHipDeg: Double,
  )

  private val handler = Handler(Looper.getMainLooper())
  private val frames = mutableListOf<AngleFrame>()
  private var movementType: String = "squat"
  private var running: Boolean = false
  private var phase: Double = 0.0

  private val frameEmitter =
    object : Runnable {
      override fun run() {
        if (!running) {
          return
        }

        val frame = buildSyntheticFrame()
        frames.add(frame)
        if (frames.size > 900) {
          frames.removeAt(0)
        }

        emitFrame(frame)
        handler.postDelayed(this, FRAME_INTERVAL_MS)
      }
    }

  init {
    reactContext.addLifecycleEventListener(this)
  }

  override fun getName(): String = "FormCheckPoseModule"

  @ReactMethod
  fun startDetection(movementType: String?, promise: Promise) {
    this.movementType = movementType?.trim().takeUnless { it.isNullOrEmpty() } ?: "squat"

    frames.clear()
    phase = 0.0
    running = true
    handler.removeCallbacks(frameEmitter)
    handler.post(frameEmitter)

    promise.resolve(null)
  }

  @ReactMethod
  fun stopDetection(promise: Promise) {
    running = false
    handler.removeCallbacks(frameEmitter)

    val summary = buildSummaryMap()
    frames.clear()

    promise.resolve(summary)
  }

  @ReactMethod
  fun addListener(eventName: String) {
    // Required for RN event emitter compatibility.
  }

  @ReactMethod
  fun removeListeners(count: Double) {
    // Required for RN event emitter compatibility.
  }

  private fun buildSyntheticFrame(): AngleFrame {
    val depth = (sin(phase) + 1.0) / 2.0
    val sway = sin(phase * 0.5)

    val frame =
      AngleFrame(
        timestampMs = System.currentTimeMillis(),
        leftKneeDeg = 173.0 - depth * 92.0 + sway * 3.0,
        rightKneeDeg = 171.0 - depth * 90.0 - sway * 3.0,
        leftHipDeg = 169.0 - depth * 78.0 + sway * 2.0,
        rightHipDeg = 167.0 - depth * 80.0 - sway * 2.0,
      )

    phase += 0.22
    return frame
  }

  private fun emitFrame(frame: AngleFrame) {
    val payload =
      Arguments.createMap().apply {
        putDouble("timestampMs", frame.timestampMs.toDouble())
        putDouble("leftKneeDeg", frame.leftKneeDeg)
        putDouble("rightKneeDeg", frame.rightKneeDeg)
        putDouble("leftHipDeg", frame.leftHipDeg)
        putDouble("rightHipDeg", frame.rightHipDeg)
      }

    reactContext
      .getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter::class.java)
      .emit(FRAME_EVENT_NAME, payload)
  }

  private fun buildSummaryMap() = Arguments.createMap().apply {
    if (frames.size < 5) {
      putString("movementType", movementType)
      putInt("sampleCount", frames.size)
      putInt("repetitionCount", 0)
      putDouble("rangeOfMotionDegrees", 0.0)
      putInt("rangeOfMotionScore", 0)
      putInt("kneeTrackingScore", 0)
      putInt("symmetryScore", 0)
      putInt("overallScore", 0)
      putArray("feedback", Arguments.fromList(listOf("Record a longer set for a reliable form check.")))
      putDouble("minLeftKneeDeg", 0.0)
      putDouble("minRightKneeDeg", 0.0)
      putDouble("maxLeftKneeDeg", 0.0)
      putDouble("maxRightKneeDeg", 0.0)
      return@apply
    }

    var minLeft = Double.POSITIVE_INFINITY
    var maxLeft = Double.NEGATIVE_INFINITY
    var minRight = Double.POSITIVE_INFINITY
    var maxRight = Double.NEGATIVE_INFINITY

    var kneeGapTotal = 0.0
    var repCount = 0
    var inBottomPosition = false

    for (frame in frames) {
      minLeft = min(minLeft, frame.leftKneeDeg)
      maxLeft = max(maxLeft, frame.leftKneeDeg)
      minRight = min(minRight, frame.rightKneeDeg)
      maxRight = max(maxRight, frame.rightKneeDeg)

      kneeGapTotal += abs(frame.leftKneeDeg - frame.rightKneeDeg)

      val averageKnee = (frame.leftKneeDeg + frame.rightKneeDeg) / 2.0
      val kneeDepth = 180.0 - averageKnee
      if (!inBottomPosition && kneeDepth >= 55.0) {
        inBottomPosition = true
        repCount += 1
      } else if (inBottomPosition && kneeDepth <= 24.0) {
        inBottomPosition = false
      }
    }

    val leftRom = maxLeft - minLeft
    val rightRom = maxRight - minRight
    val averageRom = (leftRom + rightRom) / 2.0
    val averageKneeGap = kneeGapTotal / frames.size

    val romDelta = abs(leftRom - rightRom)
    val depthDelta = abs(minLeft - minRight)

    val rangeOfMotionScore = clampToScore((averageRom / 95.0 * 100.0).roundToInt())
    val kneeTrackingScore = clampToScore((100.0 - averageKneeGap * 2.2).roundToInt())
    val symmetryScore = clampToScore((100.0 - romDelta * 2.4 - depthDelta * 1.2).roundToInt())
    val overallScore =
      clampToScore(
        (rangeOfMotionScore * 0.45 + kneeTrackingScore * 0.30 + symmetryScore * 0.25)
          .roundToInt(),
      )

    val feedback = mutableListOf<String>()
    if (averageRom < 50.0) {
      feedback.add("Increase depth to improve squat range of motion.")
    }
    if (averageKneeGap > 15.0) {
      feedback.add("Keep knees tracking evenly over the mid-foot.")
    }
    if (symmetryScore < 70) {
      feedback.add("Work on left/right symmetry and controlled descent.")
    }
    if (feedback.isEmpty()) {
      feedback.add("Solid rep quality across depth, tracking, and symmetry.")
    }

    putString("movementType", movementType)
    putInt("sampleCount", frames.size)
    putInt("repetitionCount", repCount)
    putDouble("rangeOfMotionDegrees", round1(averageRom))
    putInt("rangeOfMotionScore", rangeOfMotionScore)
    putInt("kneeTrackingScore", kneeTrackingScore)
    putInt("symmetryScore", symmetryScore)
    putInt("overallScore", overallScore)
    putArray("feedback", Arguments.fromList(feedback))
    putDouble("minLeftKneeDeg", round1(minLeft))
    putDouble("minRightKneeDeg", round1(minRight))
    putDouble("maxLeftKneeDeg", round1(maxLeft))
    putDouble("maxRightKneeDeg", round1(maxRight))
  }

  private fun clampToScore(value: Int): Int = value.coerceIn(0, 100)

  private fun round1(value: Double): Double = (value * 10.0).roundToInt() / 10.0

  override fun onHostResume() {
    // No-op.
  }

  override fun onHostPause() {
    // No-op.
  }

  override fun onHostDestroy() {
    running = false
    handler.removeCallbacks(frameEmitter)
  }

  override fun invalidate() {
    reactContext.removeLifecycleEventListener(this)
    running = false
    handler.removeCallbacks(frameEmitter)
    super.invalidate()
  }
}
