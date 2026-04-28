package com.smsnotifier.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.provider.Telephony
import java.time.Instant
import java.time.ZoneOffset

class SmsReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        if (intent.action != Telephony.Sms.Intents.SMS_RECEIVED_ACTION) return

        val messages = Telephony.Sms.Intents.getMessagesFromIntent(intent)
        for (sms in messages) {
            val data = mapOf(
                "sender" to sms.originatingAddress,
                "content" to sms.messageBody,
                "received_at" to Instant.now().atOffset(ZoneOffset.UTC).toString()
            )
            WebSocketService.sendSms(data)
        }
    }
}
