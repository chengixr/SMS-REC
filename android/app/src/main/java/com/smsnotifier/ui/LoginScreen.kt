package com.smsnotifier.ui

import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import com.smsnotifier.data.ApiClient
import kotlinx.coroutines.launch

@Composable
fun LoginScreen(
    apiClient: ApiClient,
    onLoginSuccess: (String) -> Unit
) {
    var username by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var error by remember { mutableStateOf("") }
    val scope = rememberCoroutineScope()

    Column(
        modifier = Modifier.fillMaxSize().padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.Center
    ) {
        Text("SMS Notifier", style = MaterialTheme.typography.headlineMedium)
        Spacer(modifier = Modifier.height(32.dp))
        OutlinedTextField(value = username, onValueChange = { username = it },
            label = { Text("用户名") }, modifier = Modifier.fillMaxWidth())
        Spacer(modifier = Modifier.height(8.dp))
        OutlinedTextField(value = password, onValueChange = { password = it },
            label = { Text("密码") }, modifier = Modifier.fillMaxWidth())
        if (error.isNotEmpty()) {
            Text(error, color = MaterialTheme.colorScheme.error)
            Spacer(modifier = Modifier.height(8.dp))
        }
        Spacer(modifier = Modifier.height(16.dp))
        Button(onClick = {
            scope.launch {
                val token = apiClient.login(username, password)
                if (token != null) {
                    onLoginSuccess(token)
                } else {
                    error = "登录失败"
                }
            }
        }, modifier = Modifier.fillMaxWidth()) { Text("登录") }

        Spacer(modifier = Modifier.height(8.dp))
        OutlinedButton(onClick = {
            scope.launch {
                if (apiClient.register(username, password)) {
                    val token = apiClient.login(username, password)
                    if (token != null) onLoginSuccess(token)
                    else error = "注册成功，但登录失败"
                } else {
                    error = "注册失败（用户名可能已被占用）"
                }
            }
        }, modifier = Modifier.fillMaxWidth()) { Text("注册") }
    }
}
