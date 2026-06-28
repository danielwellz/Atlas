package com.atlasmobile.unity

import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Assert.assertFalse
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class UnityHostActivitySmokeTest {
  @Test
  fun launchAndCloseUnityHostActivity() {
    ActivityScenario.launch(UnityHostActivity::class.java).use { scenario ->
      scenario.onActivity { activity ->
        assertFalse(activity.isFinishing)
        activity.finish()
      }
    }
  }
}
