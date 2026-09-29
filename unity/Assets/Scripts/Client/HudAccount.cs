using Game.V1;
using UnityEngine;

namespace Game.Client
{
    /// <summary>
    /// The account half of the HUD: log in, or create an account, before the create/join panel.
    /// The two forms swap with a button; a remembered username opens the login form with it filled
    /// in, and a first launch opens account creation.
    /// </summary>
    public partial class Hud
    {
        // bcrypt only uses the first 72 bytes, and the server rejects longer passwords.
        private const int MaxPasswordLength = 72;

        private bool _accountFormReady;
        private bool _creatingAccount;
        private string _usernameInput = "";
        private string _passwordInput = "";
        private string _repeatInput = "";
        private string _accountError;
        private GUIStyle _switchButton;

        private void OnLoginResult(LoginResult r)
        {
            _accountError = r.Rejection == LoginRejection.Unspecified ? null : LoginRejectionText(r.Rejection);
            ClearPasswords();
        }

        private void OnAccountCreateResult(AccountCreateResult r)
        {
            _accountError = r.Rejection == AccountCreateRejection.Unspecified ? null : AccountRejectionText(r.Rejection);
            ClearPasswords();
        }

        // Back to a fresh login form with the remembered username, and no code left from the lobby.
        private void OnLoggedOut()
        {
            _accountFormReady = false;
            _accountError = null;
            _codeInput = "";
            ClearPasswords();
        }

        private void ClearPasswords()
        {
            _passwordInput = "";
            _repeatInput = "";
        }

        private void DrawAccount(float screenW, float screenH)
        {
            if (_title == null) CreateLobbyStyles();
            if (_switchButton == null) _switchButton = new GUIStyle(GUI.skin.button) { fontSize = 12 };
            if (!_accountFormReady)
            {
                _usernameInput = _client.Username;
                _creatingAccount = _usernameInput.Length == 0;
                _accountFormReady = true;
            }

            float height = LobbyRowHeight * (_creatingAccount ? 11f : 9f) + 70f;
            var rect = new Rect((screenW - LobbyWidth) / 2f, (screenH - height) / 2f, LobbyWidth, height);

            GUI.color = Color.white;
            GUI.DrawTexture(rect, _slotBorder);
            GUI.DrawTexture(new Rect(rect.x + 2f, rect.y + 2f, rect.width - 4f, rect.height - 4f), _slotFill);

            float x = rect.x + 20f;
            float width = rect.width - 40f;
            float y = rect.y + 16f;

            _title.normal.textColor = LabelGold;
            GUI.Label(new Rect(x, y, width, LobbyRowHeight), "CREWMATE MARKETS", _title);
            y += LobbyRowHeight + 8f;

            if (!_client.IsConnected)
            {
                _lobbyText.normal.textColor = Dim;
                GUI.Label(new Rect(x, y, width, LobbyRowHeight), "Connecting to the server...", _lobbyText);
                return;
            }

            bool waiting = _client.AwaitingAccountReply;
            GUI.enabled = !waiting;

            _usernameInput = LabeledField(x, ref y, width, "username", "accountName", _usernameInput, false);
            _passwordInput = LabeledField(x, ref y, width, "password", "accountPassword", _passwordInput, true);
            if (_creatingAccount)
            {
                _repeatInput = LabeledField(x, ref y, width, "repeat password", "accountRepeat", _repeatInput, true);
            }
            y += 4f;

            string username = _usernameInput.Trim();
            bool mismatch = _creatingAccount && _repeatInput.Length > 0 && _repeatInput != _passwordInput;
            bool ready = username.Length >= MinNameLength
                         && _passwordInput.Length > 0
                         && (!_creatingAccount || _repeatInput == _passwordInput);

            bool submit = Event.current.type == EventType.KeyDown
                          && Event.current.keyCode == KeyCode.Return
                          && GUI.GetNameOfFocusedControl().StartsWith("account");

            GUI.enabled = ready && !waiting;
            string action = waiting ? "Please wait..." : _creatingAccount ? "Create account" : "Log in";
            if (GUI.Button(new Rect(x, y, width, LobbyRowHeight), action, _button) || (submit && GUI.enabled))
            {
                _accountError = null;
                if (_creatingAccount) _client.SendCreateAccount(username, _passwordInput);
                else _client.SendLogin(username, _passwordInput);
            }
            y += LobbyRowHeight + 10f;

            GUI.enabled = !waiting;
            string other = _creatingAccount ? "I already have an account" : "Create a new account";
            if (GUI.Button(new Rect(x, y, width, LobbyRowHeight), other, _switchButton))
            {
                _creatingAccount = !_creatingAccount;
                _accountError = null;
                ClearPasswords();
            }
            GUI.enabled = true;
            y += LobbyRowHeight + 10f;

            string error = mismatch ? "Passwords don't match" : _accountError;
            if (error != null)
            {
                _lobbyText.normal.textColor = Bad;
                GUI.Label(new Rect(x, y, width, LobbyRowHeight), error, _lobbyText);
            }
        }

        // A dim caption over a text field, advancing y past both. Password fields mask the text.
        private string LabeledField(float x, ref float y, float width, string caption, string control, string value, bool password)
        {
            _lobbyText.normal.textColor = Dim;
            GUI.Label(new Rect(x, y, width, LobbyRowHeight), caption, _lobbyText);
            y += LobbyRowHeight;

            var field = new Rect(x, y, width, LobbyRowHeight);
            GUI.SetNextControlName(control);
            value = password
                ? GUI.PasswordField(field, value, '*', MaxPasswordLength, _codeField)
                : GUI.TextField(field, value, NameLength, _codeField);
            y += LobbyRowHeight + 6f;
            return value;
        }

        private static string LoginRejectionText(LoginRejection reason) => reason switch
        {
            LoginRejection.InvalidCredentials => "Wrong username or password",
            LoginRejection.ServerError => "Server error, try again",
            _ => "Could not log in",
        };

        private static string AccountRejectionText(AccountCreateRejection reason) => reason switch
        {
            AccountCreateRejection.UsernameTaken => "That username is taken",
            AccountCreateRejection.UsernameTooShort => "Username is too short",
            AccountCreateRejection.UsernameTooLong => "Username is too long",
            AccountCreateRejection.UsernameInvalidCharacters => "Username has characters that aren't allowed",
            AccountCreateRejection.PasswordTooShort => "Password is too short",
            AccountCreateRejection.PasswordTooLong => "Password is too long",
            AccountCreateRejection.PasswordTooCommon => "Password is too common",
            AccountCreateRejection.PasswordMissingNumbers => "Password needs a number",
            AccountCreateRejection.PasswordMissingSpecialCharacters => "Password needs a special character",
            AccountCreateRejection.ServerError => "Server error, try again",
            _ => "Could not create the account",
        };
    }
}
