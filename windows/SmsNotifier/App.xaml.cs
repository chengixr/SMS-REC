using System.IO;
using System.Text.Json;
using System.Windows;
using SmsNotifier.Services;
using SmsNotifier.Views;

namespace SmsNotifier;

public partial class App
{
    private const string ConfigPath = "config.json";

    protected override void OnStartup(StartupEventArgs e)
    {
        var api = new ApiService("http://localhost:8080");
        string? token = null;

        if (File.Exists(ConfigPath))
        {
            var json = File.ReadAllText(ConfigPath);
            var config = JsonSerializer.Deserialize<Dictionary<string, string>>(json);
            if (config != null && config.TryGetValue("token", out var savedToken))
            {
                token = savedToken;
                api.SetToken(token);
            }
        }

        if (token == null)
        {
            var loginWindow = new LoginWindow(api);
            if (loginWindow.ShowDialog() == true)
            {
                token = loginWindow.Token;
                var config = new Dictionary<string, string> { ["token"] = token! };
                File.WriteAllText(ConfigPath, JsonSerializer.Serialize(config));
            }
            else
            {
                Shutdown();
                return;
            }
        }

        var mainWindow = new MainWindow(api, token!);
        mainWindow.Show();
    }
}
