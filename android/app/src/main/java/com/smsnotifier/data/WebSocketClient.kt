package com.smsnotifier.data

import com.google.gson.Gson
import com.smsnotifier.model.SmsItem
import com.smsnotifier.model.WsMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import okhttp3.*

class WebSocketClient(
    private val serverUrl: String,
    private val token: String,
    private val deviceType: String,
    private val deviceName: String
) {
    private val client = OkHttpClient()
    private var ws: WebSocket? = null
    private val gson = Gson()

    private val _connectionStatus = MutableStateFlow(false)
    val connectionStatus: StateFlow<Boolean> = _connectionStatus

    private val _incomingSms = MutableStateFlow<SmsItem?>(null)
    val incomingSms: StateFlow<SmsItem?> = _incomingSms

    fun connect() {
        val url = "${serverUrl}/ws?token=$token&device=$deviceType&name=$deviceName"
            .replace("http://", "ws://").replace("https://", "wss://")
        val req = Request.Builder().url(url).build()
        ws = client.newWebSocket(req, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                _connectionStatus.value = true
            }
            override fun onMessage(webSocket: WebSocket, text: String) {
                val msg = gson.fromJson(text, WsMessage::class.java)
                when (msg.type) {
                    "sms_deliver" -> {
                        @Suppress("UNCHECKED_CAST")
                        val data = msg.data as? Map<String, Any> ?: return
                        _incomingSms.value = SmsItem(
                            id = (data["id"] as Double).toLong(),
                            sender = data["sender"] as? String ?: "",
                            content = data["content"] as? String ?: "",
                            received_at = data["received_at"] as? String ?: ""
                        )
                    }
                }
            }
            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                _connectionStatus.value = false
            }
            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                _connectionStatus.value = false
            }
        })
    }

    fun send(type: String, data: Map<String, Any>) {
        val msg = mapOf("type" to type, "data" to data)
        ws?.send(gson.toJson(msg))
    }

    fun disconnect() {
        ws?.close(1000, "user close")
        _connectionStatus.value = false
    }
}
