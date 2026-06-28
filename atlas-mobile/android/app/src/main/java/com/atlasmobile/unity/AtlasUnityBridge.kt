package com.atlasmobile.unity

class AtlasUnityBridge {
  companion object {
    @JvmStatic
    fun sendMessageToReact(topic: String?, payload: String?) {
      UnityBridgeRuntime.receiveMessageFromUnity(topic = topic, payload = payload)
    }
  }
}
