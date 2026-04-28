package com.smsnotifier

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.core.app.ActivityCompat
import com.smsnotifier.data.ApiClient
import com.smsnotifier.service.WebSocketService
import com.smsnotifier.ui.LoginScreen
import com.smsnotifier.ui.MainScreen
import com.smsnotifier.ui.theme.SmsNotifierTheme

class MainActivity : ComponentActivity() {
    private val apiClient = ApiClient("http://10.0.2.2:8080")

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        requestPermissions()

        val prefs = getSharedPreferences("sms_prefs", 0)
        val savedToken = prefs.getString("token", "")

        setContent {
            SmsNotifierTheme {
                if (savedToken.isNullOrEmpty()) {
                    LoginScreen(apiClient) { token ->
                        prefs.edit().putString("token", token).apply()
                        apiClient.setToken(token)
                        startService()
                        setContent {
                            SmsNotifierTheme { MainScreen(apiClient) }
                        }
                    }
                } else {
                    apiClient.setToken(savedToken)
                    startService()
                    MainScreen(apiClient)
                }
            }
        }
    }

    private fun startService() {
        val intent = Intent(this, WebSocketService::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }

    private fun requestPermissions() {
        val perms = mutableListOf<String>()
        if (ActivityCompat.checkSelfPermission(this, Manifest.permission.RECEIVE_SMS)
            != PackageManager.PERMISSION_GRANTED) perms.add(Manifest.permission.RECEIVE_SMS)
        if (ActivityCompat.checkSelfPermission(this, Manifest.permission.READ_SMS)
            != PackageManager.PERMISSION_GRANTED) perms.add(Manifest.permission.READ_SMS)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
            ActivityCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS)
            != PackageManager.PERMISSION_GRANTED)
            perms.add(Manifest.permission.POST_NOTIFICATIONS)
        if (perms.isNotEmpty()) {
            ActivityCompat.requestPermissions(this, perms.toTypedArray(), 100)
        }
    }
}
