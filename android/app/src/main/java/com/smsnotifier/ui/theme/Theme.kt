package com.smsnotifier.ui.theme

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable

private val LightColors = lightColorScheme()

@Composable
fun SmsNotifierTheme(content: @Composable () -> Unit) {
    MaterialTheme(colorScheme = LightColors, content = content)
}
