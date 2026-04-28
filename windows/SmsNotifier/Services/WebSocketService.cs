using System.Net.WebSockets;
using System.Text;
using System.Text.Json;
using System.Threading;
using SmsNotifier.Models;

namespace SmsNotifier.Services;

public class WebSocketService
{
    private readonly string _serverUrl;
    private readonly string _token;
    private readonly string _deviceType;
    private readonly string _deviceName;
    private ClientWebSocket? _ws;
    private CancellationTokenSource? _cts;

    public event Action<SmsItem>? OnSmsReceived;
    public event Action<ConnectionStatus>? OnStatusChanged;

    public WebSocketService(string serverUrl, string token, string deviceType, string deviceName)
    {
        _serverUrl = serverUrl; _token = token;
        _deviceType = deviceType; _deviceName = deviceName;
    }

    public async Task ConnectAsync()
    {
        OnStatusChanged?.Invoke(ConnectionStatus.Connecting);
        _cts = new CancellationTokenSource();
        _ws = new ClientWebSocket();

        var url = _serverUrl.Replace("http://", "ws://").Replace("https://", "wss://");
        var uri = new Uri($"{url}/ws?token={_token}&device={_deviceType}&name={_deviceName}");

        try
        {
            await _ws.ConnectAsync(uri, _cts.Token);
            OnStatusChanged?.Invoke(ConnectionStatus.Connected);
            _ = ReceiveLoop();
        }
        catch
        {
            OnStatusChanged?.Invoke(ConnectionStatus.Disconnected);
        }
    }

    private async Task ReceiveLoop()
    {
        var buffer = new byte[4096];
        try
        {
            while (_ws?.State == WebSocketState.Open)
            {
                var result = await _ws.ReceiveAsync(buffer, _cts!.Token);
                if (result.MessageType == WebSocketMessageType.Close) break;

                var text = Encoding.UTF8.GetString(buffer, 0, result.Count);
                var msg = JsonSerializer.Deserialize<WsMessage>(text);
                if (msg?.Type == "sms_deliver" && msg.Data.HasValue)
                {
                    var sms = JsonSerializer.Deserialize<SmsItem>(msg.Data.Value.GetRawText());
                    if (sms != null) OnSmsReceived?.Invoke(sms);
                }
            }
        }
        catch { }
        finally
        {
            OnStatusChanged?.Invoke(ConnectionStatus.Disconnected);
        }
    }

    public async Task DisconnectAsync()
    {
        _cts?.Cancel();
        if (_ws?.State == WebSocketState.Open)
            await _ws.CloseAsync(WebSocketCloseStatus.NormalClosure, "", CancellationToken.None);
        OnStatusChanged?.Invoke(ConnectionStatus.Disconnected);
    }
}
