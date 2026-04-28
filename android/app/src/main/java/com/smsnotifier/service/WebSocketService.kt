package com.smsnotifier.service

import android.app.*
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import com.smsnotifier.data.WebSocketClient

class WebSocketService : Service() {
    companion object {
        private var wsClient: WebSocketClient? = null
        const val CHANNEL_ID = "sms_ws_channel"
        const val NOTIFICATION_ID = 1

        fun sendSms(data: Map<String, Any>) {
            wsClient?.send("sms_received", data)
        }

        fun isConnected(): Boolean = wsClient?.connectionStatus?.value ?: false
    }

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
        val notification = NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle("SMS Notifier")
            .setContentText("短信转发服务运行中")
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setOngoing(true)
            .build()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            startForeground(NOTIFICATION_ID, notification,
                android.content.pm.ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val prefs = getSharedPreferences("sms_prefs", Context.MODE_PRIVATE)
        val token = prefs.getString("token", "") ?: ""
        val serverUrl = prefs.getString("server_url", "http://10.0.2.2:8080") ?: ""
        val deviceName = Build.MODEL

        wsClient = WebSocketClient(serverUrl, token, "android", deviceName)
        wsClient?.connect()
        return START_STICKY
    }

    override fun onDestroy() {
        wsClient?.disconnect()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID, "SMS 转发服务",
                NotificationManager.IMPORTANCE_LOW
            )
            getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
        }
    }
}
