package com.smsnotifier.data

import com.google.gson.Gson
import com.smsnotifier.model.*
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.util.concurrent.TimeUnit

class ApiClient(private val baseUrl: String) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(10, TimeUnit.SECONDS)
        .build()
    private val gson = Gson()
    private var token: String? = null

    fun setToken(t: String) { token = t }

    suspend fun register(username: String, password: String): Boolean = withContext(Dispatchers.IO) {
        val body = gson.toJson(RegisterRequest(username, password))
            .toRequestBody("application/json".toMediaType())
        val req = Request.Builder().url("$baseUrl/api/register").post(body).build()
        val resp = client.newCall(req).execute()
        resp.isSuccessful
    }

    suspend fun login(username: String, password: String): String? = withContext(Dispatchers.IO) {
        val body = gson.toJson(LoginRequest(username, password))
            .toRequestBody("application/json".toMediaType())
        val req = Request.Builder().url("$baseUrl/api/login").post(body).build()
        val resp = client.newCall(req).execute()
        if (resp.isSuccessful) {
            val loginResp = gson.fromJson(resp.body?.string(), LoginResponse::class.java)
            token = loginResp.token
            loginResp.token
        } else null
    }

    suspend fun getDevices(): List<DeviceInfo> = withContext(Dispatchers.IO) {
        val req = Request.Builder().url("$baseUrl/api/devices")
            .header("Authorization", "Bearer $token").build()
        val resp = client.newCall(req).execute()
        gson.fromJson(resp.body?.string(), Array<DeviceInfo>::class.java).toList()
    }

    suspend fun getSmsHistory(page: Int = 1, size: Int = 20): List<SmsItem> = withContext(Dispatchers.IO) {
        val req = Request.Builder().url("$baseUrl/api/sms/history?page=$page&size=$size")
            .header("Authorization", "Bearer $token").build()
        val resp = client.newCall(req).execute()
        gson.fromJson(resp.body?.string(), Array<SmsItem>::class.java).toList()
    }
}
