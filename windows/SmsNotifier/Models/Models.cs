using System.Text.Json.Serialization;

namespace SmsNotifier.Models;

public class RegisterRequest
{
    [JsonPropertyName("username")] public string Username { get; set; } = "";
    [JsonPropertyName("password")] public string Password { get; set; } = "";
}

public class LoginRequest
{
    [JsonPropertyName("username")] public string Username { get; set; } = "";
    [JsonPropertyName("password")] public string Password { get; set; } = "";
}

public class LoginResponse
{
    [JsonPropertyName("token")] public string Token { get; set; } = "";
}

public class SmsItem
{
    [JsonPropertyName("id")] public long Id { get; set; }
    [JsonPropertyName("sender")] public string Sender { get; set; } = "";
    [JsonPropertyName("content")] public string Content { get; set; } = "";
    [JsonPropertyName("received_at")] public string ReceivedAt { get; set; } = "";
}

public class WsMessage
{
    [JsonPropertyName("type")] public string Type { get; set; } = "";
    [JsonPropertyName("data")] public System.Text.Json.JsonElement? Data { get; set; }
}

public enum ConnectionStatus { Connected, Disconnected, Connecting }
