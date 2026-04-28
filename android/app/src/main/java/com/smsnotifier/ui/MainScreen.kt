package com.smsnotifier.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import com.smsnotifier.data.ApiClient
import com.smsnotifier.model.ConnectionStatus
import com.smsnotifier.model.SmsItem
import com.smsnotifier.service.WebSocketService
import kotlinx.coroutines.delay

@Composable
fun MainScreen(apiClient: ApiClient) {
    var status by remember { mutableStateOf(ConnectionStatus.CONNECTING) }
    var smsList by remember { mutableStateOf(listOf<SmsItem>()) }

    LaunchedEffect(Unit) {
        while (true) {
            status = if (WebSocketService.isConnected())
                ConnectionStatus.CONNECTED else ConnectionStatus.DISCONNECTED
            try {
                smsList = apiClient.getSmsHistory(1, 50)
            } catch (_: Exception) {}
            delay(5000)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("SMS Notifier") },
                actions = {
                    Row(verticalAlignment = Alignment.CenterVertically,
                        modifier = Modifier.padding(end = 12.dp)) {
                        Box(modifier = Modifier.size(10.dp).clip(CircleShape)
                            .background(if (status == ConnectionStatus.CONNECTED)
                                Color(0xFF4CAF50) else Color(0xFFF44336)))
                        Spacer(modifier = Modifier.width(6.dp))
                        Text(if (status == ConnectionStatus.CONNECTED) "已连接" else "已断开",
                            style = MaterialTheme.typography.bodySmall)
                    }
                }
            )
        }
    ) { padding ->
        Column(modifier = Modifier.padding(padding)) {
            LinearProgressIndicator(
                modifier = Modifier.fillMaxWidth(),
                isIndeterminate = status == ConnectionStatus.CONNECTING
            )
            LazyColumn(modifier = Modifier.fillMaxSize()) {
                items(smsList) { sms ->
                    Card(modifier = Modifier.fillMaxWidth()
                        .padding(horizontal = 12.dp, vertical = 4.dp)) {
                        Column(modifier = Modifier.padding(12.dp)) {
                            Text(sms.sender, style = MaterialTheme.typography.titleSmall)
                            Text(sms.content, style = MaterialTheme.typography.bodyMedium)
                            Text(sms.received_at, style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.outline)
                        }
                    }
                }
            }
        }
    }
}
