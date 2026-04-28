using System.Net.Http;
using System.Net.Http.Json;
using SmsNotifier.Models;

namespace SmsNotifier.Services;

public class ApiService
{
    private readonly HttpClient _client;
    private string? _token;

    public string BaseUrl { get; private set; }

    public ApiService(string baseUrl)
    {
        BaseUrl = baseUrl;
        _client = new HttpClient();
    }

    public void SetBaseUrl(string url)
    {
        BaseUrl = url.TrimEnd('/');
    }

    public void SetToken(string t) => _token = t;

    public async Task<bool> Register(string username, string password)
    {
        var resp = await _client.PostAsJsonAsync($"{BaseUrl}/api/register",
            new RegisterRequest { Username = username, Password = password });
        return resp.IsSuccessStatusCode;
    }

    public async Task<string?> Login(string username, string password)
    {
        var resp = await _client.PostAsJsonAsync($"{BaseUrl}/api/login",
            new LoginRequest { Username = username, Password = password });
        if (!resp.IsSuccessStatusCode) return null;
        var result = await resp.Content.ReadFromJsonAsync<LoginResponse>();
        _token = result?.Token;
        return _token;
    }

    public async Task<List<SmsItem>> GetSmsHistory(int page = 1, int size = 50)
    {
        var req = new HttpRequestMessage(HttpMethod.Get, $"{BaseUrl}/api/sms/history?page={page}&size={size}");
        req.Headers.Add("Authorization", $"Bearer {_token}");
        var resp = await _client.SendAsync(req);
        return await resp.Content.ReadFromJsonAsync<List<SmsItem>>() ?? new List<SmsItem>();
    }
}
