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
        LoginBtn.IsEnabled = false;
        RegisterBtn.IsEnabled = false;
        try
        {
            var token = await _api.Login(UsernameBox.Text, PasswordBox.Password);
            if (token != null)
            {
                Token = token;
                DialogResult = true;
                Close();
            }
            else
            {
                ErrorText.Text = "登录失败，请检查用户名和密码";
                ErrorText.Visibility = Visibility.Visible;
            }
        }
        catch (Exception ex)
        {
            ErrorText.Text = $"连接失败：{ex.Message}";
            ErrorText.Visibility = Visibility.Visible;
        }
        finally
        {
            LoginBtn.IsEnabled = true;
            RegisterBtn.IsEnabled = true;
        }
    }

    private async void RegisterBtn_Click(object sender, RoutedEventArgs e)
    {
        ErrorText.Visibility = Visibility.Collapsed;
        LoginBtn.IsEnabled = false;
        RegisterBtn.IsEnabled = false;
        try
        {
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
                ErrorText.Text = "注册成功但登录失败，请重试";
                ErrorText.Visibility = Visibility.Visible;
            }
            else
            {
                ErrorText.Text = "注册失败（用户名可能已被占用）";
                ErrorText.Visibility = Visibility.Visible;
            }
        }
        catch (Exception ex)
        {
            ErrorText.Text = $"连接失败：{ex.Message}";
            ErrorText.Visibility = Visibility.Visible;
        }
        finally
        {
            LoginBtn.IsEnabled = true;
            RegisterBtn.IsEnabled = true;
        }
    }
}
