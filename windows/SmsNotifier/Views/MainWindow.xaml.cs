using System.Collections.ObjectModel;
using System.Windows;
using System.Windows.Media;
using Brushes = System.Windows.Media.Brushes;
using SmsNotifier.Models;
using SmsNotifier.Services;

namespace SmsNotifier.Views;

public partial class MainWindow : Window
{
    private readonly ApiService _api;
    private readonly WebSocketService _ws;
    private readonly TrayIcon _trayIcon;
    public ObservableCollection<SmsItem> SmsItems { get; } = new();

    public MainWindow(ApiService api, string token, string serverUrl)
    {
        InitializeComponent();
        _api = api;
        _api.SetToken(token);
        SmsListView.ItemsSource = SmsItems;

        _trayIcon = new TrayIcon(this);

        _ws = new WebSocketService(serverUrl, token, "windows",
            Environment.MachineName);
        _ws.OnSmsReceived += sms =>
        {
            Dispatcher.Invoke(() =>
            {
                SmsItems.Insert(0, sms);
                _trayIcon.ShowNotification(sms.Sender, sms.Content);
            });
        };
        _ws.OnStatusChanged += status =>
        {
            Dispatcher.Invoke(() => UpdateStatus(status));
        };
        Loaded += async (_, _) =>
        {
            await LoadHistory();
            await _ws.ConnectAsync();
        };
        Closing += async (_, _) => await _ws.DisconnectAsync();
    }

    private async Task LoadHistory()
    {
        try
        {
            var items = await _api.GetSmsHistory();
            foreach (var item in items) SmsItems.Add(item);
        }
        catch { }
    }

    private void UpdateStatus(ConnectionStatus status)
    {
        StatusDot.Fill = status switch
        {
            ConnectionStatus.Connected => Brushes.Green,
            ConnectionStatus.Disconnected => Brushes.Red,
            _ => Brushes.Orange
        };
        StatusText.Text = status switch
        {
            ConnectionStatus.Connected => "已连接",
            ConnectionStatus.Disconnected => "已断开",
            _ => "连接中..."
        };
    }
}
