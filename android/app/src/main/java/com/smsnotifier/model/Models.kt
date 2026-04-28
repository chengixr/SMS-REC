package com.smsnotifier.model

data class RegisterRequest(val username: String, val password: String)
data class LoginRequest(val username: String, val password: String)
data class LoginResponse(val token: String)

data class WsMessage(
    val type: String,
    val data: Map<String, Any>? = null
)

data class SmsItem(
    val id: Long,
    val sender: String,
    val content: String,
    val received_at: String,
    val delivered: Boolean = false
)

data class DeviceInfo(
    val id: Int,
    val device_type: String,
    val device_name: String,
    val last_seen: String
)

enum class ConnectionStatus {
    CONNECTED, DISCONNECTED, CONNECTING
}
