package com.example.myapplication

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.BackHandler
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ExitToApp
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.Person
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.example.myapplication.ui.theme.MyApplicationTheme

class MainActivity : ComponentActivity() {

    private val requestPermissionLauncher =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { /* no-op */ }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        Notifications.createChannel(this)
        requestNotificationPermission()
        enableEdgeToEdge()
        setContent {
            MyApplicationTheme {
                MessengerApp()
            }
        }
    }

    private fun requestNotificationPermission() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS)
                != PackageManager.PERMISSION_GRANTED
            ) {
                requestPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
            }
        }
    }
}

// ---- App root ----
@Composable
fun MessengerApp(vm: MessengerViewModel = viewModel()) {
    val state by vm.state.collectAsStateWithLifecycle()

    when (state.screen) {
        Screen.LOGIN -> LoginScreen(state, vm)
        Screen.CHAT  -> ChatScreen(state, vm)
    }
}

// ---- Login / Register / Settings screen ----
@Composable
fun LoginScreen(state: UiState, vm: MessengerViewModel) {
    var tab by remember { mutableIntStateOf(0) }
    var loginUser by remember { mutableStateOf("") }
    var loginPass by remember { mutableStateOf("") }
    var regUser   by remember { mutableStateOf("") }
    var regPass   by remember { mutableStateOf("") }
    var cfgServer by remember { mutableStateOf(state.config.serverAddr) }
    var cfgProxy  by remember { mutableStateOf(state.config.proxyAddr) }
    var cfgDirect by remember { mutableStateOf(state.config.directMode) }

    LaunchedEffect(state.config) {
        cfgServer = state.config.serverAddr
        cfgProxy  = state.config.proxyAddr
        cfgDirect = state.config.directMode
    }

    Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Text(
                "🔐 DNSTT Messenger",
                fontSize = 24.sp,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(bottom = 24.dp)
            )

            TabRow(selectedTabIndex = tab) {
                Tab(selected = tab == 0, onClick = { tab = 0 }, text = { Text("Вход") })
                Tab(selected = tab == 1, onClick = { tab = 1 }, text = { Text("Регистрация") })
                Tab(selected = tab == 2, onClick = { tab = 2 }, text = { Text("Настройки") })
            }

            Spacer(Modifier.height(16.dp))

            when (tab) {
                0 -> {
                    OutlinedTextField(
                        value = loginUser, onValueChange = { loginUser = it },
                        label = { Text("Логин") }, singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next)
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = loginPass, onValueChange = { loginPass = it },
                        label = { Text("Пароль") }, singleLine = true,
                        visualTransformation = PasswordVisualTransformation(),
                        modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(
                            keyboardType = KeyboardType.Password,
                            imeAction = ImeAction.Done
                        ),
                        keyboardActions = KeyboardActions(onDone = { vm.login(loginUser, loginPass) })
                    )
                    Spacer(Modifier.height(16.dp))
                    Button(
                        onClick = { vm.login(loginUser, loginPass) },
                        modifier = Modifier.fillMaxWidth(),
                        enabled = !state.isLoading
                    ) {
                        if (state.isLoading) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                        else Text("Войти")
                    }
                }
                1 -> {
                    OutlinedTextField(
                        value = regUser, onValueChange = { regUser = it },
                        label = { Text("Логин") }, singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next)
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = regPass, onValueChange = { regPass = it },
                        label = { Text("Пароль") }, singleLine = true,
                        visualTransformation = PasswordVisualTransformation(),
                        modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(
                            keyboardType = KeyboardType.Password,
                            imeAction = ImeAction.Done
                        ),
                        keyboardActions = KeyboardActions(onDone = { vm.register(regUser, regPass) })
                    )
                    Spacer(Modifier.height(16.dp))
                    Button(
                        onClick = { vm.register(regUser, regPass) },
                        modifier = Modifier.fillMaxWidth(),
                        enabled = !state.isLoading
                    ) {
                        if (state.isLoading) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                        else Text("Зарегистрироваться")
                    }
                }
                2 -> {
                    OutlinedTextField(
                        value = cfgServer, onValueChange = { cfgServer = it },
                        label = { Text("Адрес сервера") }, singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = cfgProxy, onValueChange = { cfgProxy = it },
                        label = { Text("SOCKS5 прокси (dnstt)") }, singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )
                    Spacer(Modifier.height(8.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Checkbox(checked = cfgDirect, onCheckedChange = { cfgDirect = it })
                        Text("Прямое подключение (без прокси)")
                    }
                    Spacer(Modifier.height(16.dp))
                    Button(
                        onClick = { vm.saveConfig(AppConfig(cfgServer, cfgProxy, cfgDirect)) },
                        modifier = Modifier.fillMaxWidth()
                    ) { Text("Сохранить") }
                }
            }

            if (state.status.isNotEmpty()) {
                Spacer(Modifier.height(12.dp))
                Text(
                    state.status,
                    color = if (state.isError) MaterialTheme.colorScheme.error
                            else MaterialTheme.colorScheme.primary,
                    fontSize = 14.sp
                )
            }
        }
    }
}

// ---- Chat screen ----
@Composable
fun ChatScreen(state: UiState, vm: MessengerViewModel) {
    var msgText by remember { mutableStateOf("") }
    var showOnline by remember { mutableStateOf(false) }
    val listState = rememberLazyListState()

    BackHandler { vm.logout() }

    LaunchedEffect(state.messages.size) {
        if (state.messages.isNotEmpty())
            listState.animateScrollToItem(state.messages.size - 1)
    }

    Scaffold(
        topBar = {
            @OptIn(ExperimentalMaterial3Api::class)
            TopAppBar(
                title = { Text(state.myUsername, fontWeight = FontWeight.Bold) },
                actions = {
                    IconButton(onClick = { showOnline = !showOnline }) {
                        Icon(Icons.Default.Person, contentDescription = "Онлайн")
                    }
                    IconButton(onClick = { vm.logout() }) {
                        Icon(Icons.AutoMirrored.Filled.ExitToApp, contentDescription = "Выйти")
                    }
                }
            )
        },
        bottomBar = {
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(8.dp),
                verticalAlignment = Alignment.Bottom
            ) {
                OutlinedTextField(
                    value = msgText,
                    onValueChange = { msgText = it },
                    placeholder = { Text("Сообщение...") },
                    modifier = Modifier.weight(1f),
                    maxLines = 4,
                    keyboardOptions = KeyboardOptions(imeAction = ImeAction.Send),
                    keyboardActions = KeyboardActions(onSend = {
                        vm.sendMessage(msgText); msgText = ""
                    })
                )
                Spacer(Modifier.width(8.dp))
                IconButton(
                    onClick = { vm.sendMessage(msgText); msgText = "" },
                    enabled = msgText.isNotBlank()
                ) {
                    Icon(Icons.AutoMirrored.Filled.Send, contentDescription = "Отправить",
                        tint = MaterialTheme.colorScheme.primary)
                }
            }
        }
    ) { padding ->
        Row(modifier = Modifier.padding(padding).fillMaxSize()) {
            LazyColumn(
                state = listState,
                modifier = Modifier.weight(1f).fillMaxHeight(),
                contentPadding = PaddingValues(8.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                items(state.messages) { msg ->
                    MessageBubble(msg)
                }
            }

            if (showOnline) {
                Divider(modifier = Modifier.fillMaxHeight().width(1.dp))
                Column(
                    modifier = Modifier
                        .width(120.dp)
                        .fillMaxHeight()
                        .padding(8.dp)
                ) {
                    Text("Онлайн", fontWeight = FontWeight.Bold, fontSize = 13.sp)
                    Spacer(Modifier.height(4.dp))
                    state.onlineUsers.forEach { user ->
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Box(
                                Modifier
                                    .size(8.dp)
                                    .background(Color(0xFF4CAF50), RoundedCornerShape(50))
                            )
                            Spacer(Modifier.width(4.dp))
                            Text(user, fontSize = 12.sp, maxLines = 1)
                        }
                        Spacer(Modifier.height(2.dp))
                    }
                }
            }
        }
    }
}

@Composable
fun MessageBubble(msg: ChatMessage) {
    val alignment = if (msg.own) Alignment.End else Alignment.Start
    val bubbleColor = if (msg.own) MaterialTheme.colorScheme.primaryContainer
                      else MaterialTheme.colorScheme.surfaceVariant

    Column(
        modifier = Modifier.fillMaxWidth(),
        horizontalAlignment = alignment
    ) {
        Surface(
            shape = RoundedCornerShape(
                topStart = 12.dp, topEnd = 12.dp,
                bottomStart = if (msg.own) 12.dp else 2.dp,
                bottomEnd   = if (msg.own) 2.dp  else 12.dp
            ),
            color = bubbleColor,
            modifier = Modifier.widthIn(max = 280.dp)
        ) {
            Column(modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp)) {
                if (!msg.own) {
                    Text(
                        msg.sender,
                        fontSize = 11.sp,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.primary
                    )
                }
                Text(msg.text, fontSize = 15.sp)
                Text(
                    msg.time,
                    fontSize = 10.sp,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    modifier = Modifier.align(Alignment.End)
                )
            }
        }
    }
}
