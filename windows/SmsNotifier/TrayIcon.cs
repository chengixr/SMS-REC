using System.Drawing;
using System.Windows;
using System.Windows.Forms;

namespace SmsNotifier;

public class TrayIcon
{
    private readonly NotifyIcon _notifyIcon;
    private readonly Window _window;

    public TrayIcon(Window window)
    {
        _window = window;
        _notifyIcon = new NotifyIcon
        {
            Text = "SMS Notifier",
            Icon = SystemIcons.Application,
            Visible = true
        };
        _notifyIcon.DoubleClick += (_, _) =>
        {
            _window.Show();
            _window.WindowState = WindowState.Normal;
        };
        _window.StateChanged += (_, _) =>
        {
            if (_window.WindowState == WindowState.Minimized)
                _window.Hide();
        };
    }

    public void ShowNotification(string title, string text)
    {
        _notifyIcon.ShowBalloonTip(3000, title, text, ToolTipIcon.Info);
    }
}
