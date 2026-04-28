using System.Windows;
using SmsNotifier.Services;

namespace SmsNotifier.Views;

public partial class LoginWindow : Window
{
    private readonly ApiService _api;
    public string? Token { get; private set; }

    public LoginWindow(ApiService api)
    {
        InitializeComponent();
        _api = api;
    }

    private async void LoginBtn_Click(object sender, RoutedEventArgs e)
    {
        ErrorText.Visibility = Visibility.Collapsed;
        var token = await _api.Login(UsernameBox.Text, PasswordBox.Password);
        if (token != null)
        {
            Token = token;
            DialogResult = true;
            Close();
        }
        else
        {
            ErrorText.Text = "登录失败";
            ErrorText.Visibility = Visibility.Visible;
        }
    }

    private async void RegisterBtn_Click(object sender, RoutedEventArgs e)
    {
        ErrorText.Visibility = Visibility.Collapsed;
        if (await _api.Register(UsernameBox.Text, PasswordBox.Password))
        {
            var token = await _api.Login(UsernameBox.Text, PasswordBox.Password);
            if (token != null)
            {
                Token = token;
                DialogResult = true;
                Close();
                return;
            }
        }
        ErrorText.Text = "注册失败（用户名可能已被占用）";
        ErrorText.Visibility = Visibility.Visible;
    }
}
