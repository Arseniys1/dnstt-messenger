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
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.ExitToApp
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Settings
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
import androidx.core.view.WindowCompat
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
        WindowCompat.setDecorFitsSystemWindows(window, false)
        enableEdgeToEdge()
        setContent {
            MyApplicationTheme {
                MessengerApp()
            }
        }
    }

    override fun onResume() {
        super.onResume()
        AppForegroundState.isInForeground = true
    }

    override fun onPause() {
        super.onPause()
        AppForegroundState.isInForeground = false
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
        Screen.DM    -> DMScreen(state, vm)
        Screen.ROOM  -> RoomScreen(state, vm)
        Screen.SETTINGS -> SettingsScreen(state, vm)
    }
}

data class LanguageOption(val code: String, val labelRes: Int)

private fun languageOptions(): List<LanguageOption> = listOf(
    LanguageOption("en", R.string.lang_english),
    LanguageOption("ru", R.string.lang_russian),
    LanguageOption("zh", R.string.lang_chinese_simplified),
    LanguageOption("fa", R.string.lang_persian),
    LanguageOption("tr", R.string.lang_turkish)
)

@Composable
@OptIn(ExperimentalMaterial3Api::class)
private fun SettingsForm(
    lang: String,
    state: UiState,
    onSave: (AppConfig) -> Unit,
    modifier: Modifier = Modifier
) {
    val options = languageOptions()
    var cfgServer by remember { mutableStateOf(state.config.serverAddr) }
    var cfgProxy by remember { mutableStateOf(state.config.proxyAddr) }
    var cfgDirect by remember { mutableStateOf(state.config.directMode) }
    var cfgLanguage by remember { mutableStateOf(state.config.language) }
    var languageExpanded by remember { mutableStateOf(false) }

    LaunchedEffect(state.config) {
        cfgServer = state.config.serverAddr
        cfgProxy = state.config.proxyAddr
        cfgDirect = state.config.directMode
        cfgLanguage = state.config.language
    }

    Column(modifier = modifier) {
        OutlinedTextField(
            value = cfgServer, onValueChange = { cfgServer = it },
            label = { Text(t(lang, R.string.label_server_address)) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth()
        )
        Spacer(Modifier.height(8.dp))
        OutlinedTextField(
            value = cfgProxy, onValueChange = { cfgProxy = it },
            label = { Text(t(lang, R.string.label_socks_proxy)) },
            singleLine = true,
            modifier = Modifier.fillMaxWidth()
        )
        Spacer(Modifier.height(8.dp))
        Row(verticalAlignment = Alignment.CenterVertically) {
            Checkbox(checked = cfgDirect, onCheckedChange = { cfgDirect = it })
            Text(t(lang, R.string.label_direct_connection))
        }
        Spacer(Modifier.height(8.dp))
        Text(t(lang, R.string.label_language), fontSize = 13.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        ExposedDropdownMenuBox(
            expanded = languageExpanded,
            onExpandedChange = { languageExpanded = !languageExpanded }
        ) {
            val selectedLanguage = options.firstOrNull { it.code == cfgLanguage } ?: options.first()
            OutlinedTextField(
                value = t(lang, selectedLanguage.labelRes),
                onValueChange = {},
                readOnly = true,
                singleLine = true,
                modifier = Modifier
                    .menuAnchor()
                    .fillMaxWidth(),
                trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = languageExpanded) }
            )
            ExposedDropdownMenu(
                expanded = languageExpanded,
                onDismissRequest = { languageExpanded = false }
            ) {
                options.forEach { option ->
                    DropdownMenuItem(
                        text = { Text(t(lang, option.labelRes)) },
                        onClick = {
                            cfgLanguage = option.code
                            languageExpanded = false
                        }
                    )
                }
            }
        }
        Spacer(Modifier.height(16.dp))
        Button(
            onClick = { onSave(AppConfig(cfgServer, cfgProxy, cfgDirect, cfgLanguage)) },
            modifier = Modifier.fillMaxWidth()
        ) { Text(t(lang, R.string.action_save)) }

        if (state.knownServers.isNotEmpty()) {
            Spacer(Modifier.height(16.dp))
            Text(
                t(lang, R.string.label_network_servers),
                fontSize = 13.sp,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Spacer(Modifier.height(6.dp))
            state.knownServers.forEach { addr ->
                OutlinedButton(
                    onClick = {
                        cfgServer = addr
                        cfgDirect = true
                        onSave(AppConfig(addr, cfgProxy, true, cfgLanguage))
                    },
                    modifier = Modifier.fillMaxWidth().padding(vertical = 2.dp)
                ) {
                    Text(addr, fontSize = 13.sp, maxLines = 1)
                }
            }
        }
    }
}

// ---- Login / Register / Settings screen ----
@Composable
@OptIn(ExperimentalMaterial3Api::class)
fun LoginScreen(state: UiState, vm: MessengerViewModel) {
    val lang = state.config.language
    var tab by remember { mutableIntStateOf(0) }
    var loginUser by remember { mutableStateOf("") }
    var loginPass by remember { mutableStateOf("") }
    var regUser by remember { mutableStateOf("") }
    var regPass by remember { mutableStateOf("") }

    Surface(modifier = Modifier.fillMaxSize(), color = MaterialTheme.colorScheme.background) {
        Column(
            modifier = Modifier.fillMaxSize().padding(24.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            Text(
                "DNSTT Messenger",
                fontSize = 24.sp,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(bottom = 24.dp)
            )

            TabRow(selectedTabIndex = tab) {
                Tab(selected = tab == 0, onClick = { tab = 0 }, text = { Text(t(lang, R.string.tab_login)) })
                Tab(selected = tab == 1, onClick = { tab = 1 }, text = { Text(t(lang, R.string.tab_register)) })
                Tab(selected = tab == 2, onClick = { tab = 2 }, text = { Text(t(lang, R.string.tab_settings)) })
            }

            Spacer(Modifier.height(16.dp))
            when (tab) {
                0 -> {
                    OutlinedTextField(
                        value = loginUser, onValueChange = { loginUser = it },
                        label = { Text(t(lang, R.string.label_login)) },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next)
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = loginPass, onValueChange = { loginPass = it },
                        label = { Text(t(lang, R.string.label_password)) },
                        singleLine = true,
                        visualTransformation = PasswordVisualTransformation(),
                        modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Done),
                        keyboardActions = KeyboardActions(onDone = { vm.login(loginUser, loginPass) })
                    )
                    Spacer(Modifier.height(16.dp))
                    Button(
                        onClick = { vm.login(loginUser, loginPass) },
                        modifier = Modifier.fillMaxWidth(),
                        enabled = !state.isLoading
                    ) {
                        if (state.isLoading) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                        else Text(t(lang, R.string.action_sign_in))
                    }
                }
                1 -> {
                    OutlinedTextField(
                        value = regUser, onValueChange = { regUser = it },
                        label = { Text(t(lang, R.string.label_login)) },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next)
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = regPass, onValueChange = { regPass = it },
                        label = { Text(t(lang, R.string.label_password)) },
                        singleLine = true,
                        visualTransformation = PasswordVisualTransformation(),
                        modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Done),
                        keyboardActions = KeyboardActions(onDone = { vm.register(regUser, regPass) })
                    )
                    Spacer(Modifier.height(16.dp))
                    Button(
                        onClick = { vm.register(regUser, regPass) },
                        modifier = Modifier.fillMaxWidth(),
                        enabled = !state.isLoading
                    ) {
                        if (state.isLoading) CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                        else Text(t(lang, R.string.action_create_account))
                    }
                }
                2 -> {
                    SettingsForm(
                        lang = lang,
                        state = state,
                        onSave = vm::saveConfig,
                        modifier = Modifier.fillMaxWidth()
                    )
                }
            }

            if (state.status.isNotEmpty()) {
                Spacer(Modifier.height(12.dp))
                Text(
                    state.status,
                    color = if (state.isError) MaterialTheme.colorScheme.error else MaterialTheme.colorScheme.primary,
                    fontSize = 14.sp
                )
            }
        }
    }
}

@Composable
fun SettingsScreen(state: UiState, vm: MessengerViewModel) {
    val lang = state.config.language
    BackHandler { vm.closeSettings() }

    Scaffold(
        topBar = {
            @OptIn(ExperimentalMaterial3Api::class)
            TopAppBar(
                title = { Text(t(lang, R.string.tab_settings), fontWeight = FontWeight.Bold) },
                navigationIcon = {
                    IconButton(onClick = { vm.closeSettings() }) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = t(lang, R.string.action_cancel))
                    }
                }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .padding(padding)
                .fillMaxSize()
                .padding(16.dp)
        ) {
            SettingsForm(
                lang = lang,
                state = state,
                onSave = vm::saveConfig,
                modifier = Modifier.fillMaxWidth()
            )
        }
    }
}

// ---- Chat screen ----
@Composable
fun ChatScreen(state: UiState, vm: MessengerViewModel) {
    var msgText by remember { mutableStateOf("") }
    var showSidebar by remember { mutableStateOf(false) }
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
                title = { Text(t(state.config.language, R.string.chat_general_title), fontWeight = FontWeight.Bold) },
                navigationIcon = {
                    IconButton(onClick = { showSidebar = !showSidebar }) {
                        Icon(Icons.Default.Person, contentDescription = t(state.config.language, R.string.content_menu))
                    }
                },
                actions = {
                    IconButton(onClick = { vm.openSettings() }) {
                        Icon(Icons.Default.Settings, contentDescription = t(state.config.language, R.string.tab_settings))
                    }
                    IconButton(onClick = { vm.logout() }) {
                        Icon(
                            Icons.AutoMirrored.Filled.ExitToApp,
                            contentDescription = t(state.config.language, R.string.content_logout)
                        )
                    }
                }
            )
        },
        bottomBar = {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .imePadding()
                    .navigationBarsPadding()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 8.dp, vertical = 6.dp),
                    verticalAlignment = Alignment.Bottom
                ) {
                    OutlinedTextField(
                        value = msgText,
                        onValueChange = { msgText = it },
                        placeholder = { Text(t(state.config.language, R.string.placeholder_message)) },
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
                        Icon(Icons.AutoMirrored.Filled.Send, contentDescription = t(state.config.language, R.string.content_send),
                            tint = MaterialTheme.colorScheme.primary)
                    }
                }
            }
        }
    ) { padding ->
        Row(modifier = Modifier.padding(padding).fillMaxSize()) {
            if (showSidebar) {
                NavigationSidebar(state, vm, onDismiss = { showSidebar = false })
            }
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


// ---- Navigation Sidebar ----
@Composable
fun NavigationSidebar(state: UiState, vm: MessengerViewModel, onDismiss: () -> Unit) {
    var showNewDMDialog by remember { mutableStateOf(false) }
    var showNewRoomDialog by remember { mutableStateOf(false) }

    Surface(
        modifier = Modifier
            .fillMaxHeight()
            .width(280.dp),
        color = MaterialTheme.colorScheme.surface,
        tonalElevation = 2.dp
    ) {
        Column(modifier = Modifier.fillMaxSize()) {
            // Header
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(16.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(state.myUsername, fontWeight = FontWeight.Bold, fontSize = 18.sp)
                IconButton(onClick = onDismiss) {
                    Text("✕", fontSize = 20.sp)
                }
            }
            Divider()

            LazyColumn(modifier = Modifier.weight(1f)) {
                // Global chat
                item {
                    Text(
                        t(state.config.language, R.string.label_chats),
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                item {
                    NavigationItem(
                        text = t(state.config.language, R.string.label_global_chat),
                        selected = state.screen == Screen.CHAT,
                        onClick = { vm.switchToGlobalChat(); onDismiss() }
                    )
                }
                item {
                    NavigationItem(
                        text = t(state.config.language, R.string.tab_settings),
                        selected = state.screen == Screen.SETTINGS,
                        onClick = { vm.openSettings(); onDismiss() }
                    )
                }

                // DMs
                item {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 16.dp, vertical = 8.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            t(state.config.language, R.string.label_direct_messages),
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        IconButton(
                            onClick = { showNewDMDialog = true },
                            modifier = Modifier.size(24.dp)
                        ) {
                            Text("+", fontSize = 18.sp)
                        }
                    }
                }
                items(state.dmConversations.keys.toList()) { partner ->
                    val unread = state.unreadDMs[partner] ?: 0
                    NavigationItem(
                        text = "💬 $partner",
                        selected = state.screen == Screen.DM && state.currentDMPartner == partner,
                        onClick = { vm.switchToDM(partner); onDismiss() },
                        badge = if (unread > 0) unread else null
                    )
                }

                // Rooms
                item {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 16.dp, vertical = 8.dp),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            t(state.config.language, R.string.label_rooms),
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Bold,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        IconButton(
                            onClick = { showNewRoomDialog = true },
                            modifier = Modifier.size(24.dp)
                        ) {
                            Text("+", fontSize = 18.sp)
                        }
                    }
                }
                items(state.rooms.values.toList()) { room ->
                    val unread = state.unreadRooms[room.id] ?: 0
                    val prefix = if (room.isPublic) "🌐" else "🔒"
                    NavigationItem(
                        text = "$prefix ${room.name}",
                        selected = state.screen == Screen.ROOM && state.currentRoomId == room.id,
                        onClick = { vm.switchToRoom(room.id); onDismiss() },
                        badge = if (unread > 0) unread else null
                    )
                }

                // Online users
                item {
                    Text(
                        t(state.config.language, R.string.label_online_count, state.onlineUsers.size),
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                items(state.onlineUsers) { user ->
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 16.dp, vertical = 4.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Box(
                            Modifier
                                .size(8.dp)
                                .background(Color(0xFF4CAF50), RoundedCornerShape(50))
                        )
                        Spacer(Modifier.width(8.dp))
                        Text(user, fontSize = 14.sp)
                    }
                }
            }
        }
    }

    if (showNewDMDialog) {
        NewDMDialog(
            lang = state.config.language,
            onDismiss = { showNewDMDialog = false },
            onConfirm = { username ->
                vm.switchToDM(username)
                showNewDMDialog = false
                onDismiss()
            }
        )
    }

    if (showNewRoomDialog) {
        NewRoomDialog(
            lang = state.config.language,
            onDismiss = { showNewRoomDialog = false },
            onConfirm = { name, isPublic, description ->
                vm.createRoom(name, isPublic, description)
                showNewRoomDialog = false
            }
        )
    }
}

@Composable
fun NavigationItem(
    text: String,
    selected: Boolean,
    onClick: () -> Unit,
    badge: Int? = null
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 8.dp, vertical = 2.dp),
        color = if (selected) MaterialTheme.colorScheme.primaryContainer else Color.Transparent,
        shape = RoundedCornerShape(8.dp),
        onClick = onClick
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 10.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Text(
                text,
                fontSize = 14.sp,
                fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal,
                maxLines = 1,
                modifier = Modifier.weight(1f)
            )
            if (badge != null && badge > 0) {
                Surface(
                    shape = RoundedCornerShape(10.dp),
                    color = MaterialTheme.colorScheme.error
                ) {
                    Text(
                        badge.toString(),
                        modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp),
                        fontSize = 11.sp,
                        color = MaterialTheme.colorScheme.onError,
                        fontWeight = FontWeight.Bold
                    )
                }
            }
        }
    }
}

// ---- DM Screen ----
@Composable
fun DMScreen(state: UiState, vm: MessengerViewModel) {
    var msgText by remember { mutableStateOf("") }
    var showSidebar by remember { mutableStateOf(false) }
    val listState = rememberLazyListState()
    val messages = state.dmConversations[state.currentDMPartner] ?: emptyList()

    BackHandler { vm.switchToGlobalChat() }

    LaunchedEffect(messages.size) {
        if (messages.isNotEmpty())
            listState.animateScrollToItem(messages.size - 1)
    }

    Scaffold(
        topBar = {
            @OptIn(ExperimentalMaterial3Api::class)
            TopAppBar(
                title = { Text(t(state.config.language, R.string.title_dm, state.currentDMPartner), fontWeight = FontWeight.Bold) },
                navigationIcon = {
                    IconButton(onClick = { showSidebar = !showSidebar }) {
                        Icon(Icons.Default.Person, contentDescription = t(state.config.language, R.string.content_menu))
                    }
                },
                actions = {
                    IconButton(onClick = { vm.openSettings() }) {
                        Icon(Icons.Default.Settings, contentDescription = t(state.config.language, R.string.tab_settings))
                    }
                    IconButton(onClick = { vm.switchToGlobalChat() }) {
                        Text("←", fontSize = 24.sp)
                    }
                }
            )
        },
        bottomBar = {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .imePadding()
                    .navigationBarsPadding()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 8.dp, vertical = 6.dp),
                    verticalAlignment = Alignment.Bottom
                ) {
                    OutlinedTextField(
                        value = msgText,
                        onValueChange = { msgText = it },
                        placeholder = { Text(t(state.config.language, R.string.placeholder_dm_message, state.currentDMPartner)) },
                        modifier = Modifier.weight(1f),
                        maxLines = 4,
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Send),
                        keyboardActions = KeyboardActions(onSend = {
                            vm.sendDM(state.currentDMPartner, msgText); msgText = ""
                        })
                    )
                    Spacer(Modifier.width(8.dp))
                    IconButton(
                        onClick = { vm.sendDM(state.currentDMPartner, msgText); msgText = "" },
                        enabled = msgText.isNotBlank()
                    ) {
                        Icon(Icons.AutoMirrored.Filled.Send, contentDescription = t(state.config.language, R.string.content_send),
                            tint = MaterialTheme.colorScheme.primary)
                    }
                }
            }
        }
    ) { padding ->
        Row(modifier = Modifier.padding(padding).fillMaxSize()) {
            if (showSidebar) {
                NavigationSidebar(state, vm, onDismiss = { showSidebar = false })
            }
            LazyColumn(
                state = listState,
                modifier = Modifier.weight(1f).fillMaxHeight(),
                contentPadding = PaddingValues(8.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                items(messages) { msg ->
                    MessageBubble(msg)
                }
            }
        }
    }
}

// ---- Room Screen ----
@Composable
fun RoomScreen(state: UiState, vm: MessengerViewModel) {
    var msgText by remember { mutableStateOf("") }
    var showSidebar by remember { mutableStateOf(false) }
    var showInviteDialog by remember { mutableStateOf(false) }
    val listState = rememberLazyListState()
    val room = state.rooms[state.currentRoomId]
    val messages = room?.messages ?: emptyList()

    BackHandler { vm.switchToGlobalChat() }

    LaunchedEffect(messages.size) {
        if (messages.isNotEmpty())
            listState.animateScrollToItem(messages.size - 1)
    }

    // Show loading if room is not loaded yet
    if (room == null) {
        Box(
            modifier = Modifier.fillMaxSize(),
            contentAlignment = Alignment.Center
        ) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                CircularProgressIndicator()
                Spacer(Modifier.height(16.dp))
                Text(t(state.config.language, R.string.text_loading_room))
            }
        }
        return
    }

    Scaffold(
        topBar = {
            @OptIn(ExperimentalMaterial3Api::class)
            TopAppBar(
                title = {
                    Column {
                        Text(
                            "# ${room.name}",
                            fontWeight = FontWeight.Bold,
                            fontSize = 16.sp
                        )
                        Text(
                            t(state.config.language, R.string.label_members_count, room.members.size),
                            fontSize = 11.sp,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                },
                navigationIcon = {
                    IconButton(onClick = { showSidebar = !showSidebar }) {
                        Icon(Icons.Default.Person, contentDescription = t(state.config.language, R.string.content_menu))
                    }
                },
                actions = {
                    IconButton(onClick = { vm.openSettings() }) {
                        Icon(Icons.Default.Settings, contentDescription = t(state.config.language, R.string.tab_settings))
                    }
                    IconButton(onClick = { showInviteDialog = true }) {
                        Text("+", fontSize = 24.sp)
                    }
                    IconButton(onClick = { vm.leaveRoom(state.currentRoomId); vm.switchToGlobalChat() }) {
                        Text("🚪", fontSize = 18.sp)
                    }
                }
            )
        },
        bottomBar = {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .imePadding()
                    .navigationBarsPadding()
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 8.dp, vertical = 6.dp),
                    verticalAlignment = Alignment.Bottom
                ) {
                    OutlinedTextField(
                        value = msgText,
                        onValueChange = { msgText = it },
                        placeholder = { Text(t(state.config.language, R.string.placeholder_room_message)) },
                        modifier = Modifier.weight(1f),
                        maxLines = 4,
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Send),
                        keyboardActions = KeyboardActions(onSend = {
                            vm.sendRoomMessage(state.currentRoomId, msgText); msgText = ""
                        })
                    )
                    Spacer(Modifier.width(8.dp))
                    IconButton(
                        onClick = { vm.sendRoomMessage(state.currentRoomId, msgText); msgText = "" },
                        enabled = msgText.isNotBlank()
                    ) {
                        Icon(Icons.AutoMirrored.Filled.Send, contentDescription = t(state.config.language, R.string.content_send),
                            tint = MaterialTheme.colorScheme.primary)
                    }
                }
            }
        }
    ) { padding ->
        Row(modifier = Modifier.padding(padding).fillMaxSize()) {
            if (showSidebar) {
                NavigationSidebar(state, vm, onDismiss = { showSidebar = false })
            }
            LazyColumn(
                state = listState,
                modifier = Modifier.weight(1f).fillMaxHeight(),
                contentPadding = PaddingValues(8.dp),
                verticalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                items(messages) { msg ->
                    MessageBubble(msg)
                }
            }
        }
    }

    if (showInviteDialog) {
        InviteToRoomDialog(
            lang = state.config.language,
            onDismiss = { showInviteDialog = false },
            onConfirm = { username ->
                vm.inviteToRoom(state.currentRoomId, username)
                showInviteDialog = false
            }
        )
    }
}

// ---- Dialogs ----
@Composable
fun NewDMDialog(lang: String, onDismiss: () -> Unit, onConfirm: (String) -> Unit) {
    var username by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(t(lang, R.string.title_new_dm)) },
        text = {
            OutlinedTextField(
                value = username,
                onValueChange = { username = it },
                label = { Text(t(lang, R.string.label_user_login)) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )
        },
        confirmButton = {
            Button(
                onClick = { if (username.isNotBlank()) onConfirm(username) },
                enabled = username.isNotBlank()
            ) {
                Text(t(lang, R.string.action_open))
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(t(lang, R.string.action_cancel))
            }
        }
    )
}

@Composable
fun NewRoomDialog(lang: String, onDismiss: () -> Unit, onConfirm: (String, Boolean, String) -> Unit) {
    var name by remember { mutableStateOf("") }
    var description by remember { mutableStateOf("") }
    var isPublic by remember { mutableStateOf(false) }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(t(lang, R.string.title_create_room)) },
        text = {
            Column {
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = { Text(t(lang, R.string.label_room_name)) },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(
                    value = description,
                    onValueChange = { description = it },
                    label = { Text(t(lang, R.string.label_description_optional)) },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(Modifier.height(8.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Checkbox(checked = isPublic, onCheckedChange = { isPublic = it })
                    Text(t(lang, R.string.label_room_public))
                }
            }
        },
        confirmButton = {
            Button(
                onClick = { if (name.isNotBlank()) onConfirm(name, isPublic, description) },
                enabled = name.isNotBlank()
            ) {
                Text(t(lang, R.string.action_create))
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(t(lang, R.string.action_cancel))
            }
        }
    )
}

@Composable
fun InviteToRoomDialog(lang: String, onDismiss: () -> Unit, onConfirm: (String) -> Unit) {
    var username by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(t(lang, R.string.title_invite_room)) },
        text = {
            OutlinedTextField(
                value = username,
                onValueChange = { username = it },
                label = { Text(t(lang, R.string.label_user_login)) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )
        },
        confirmButton = {
            Button(
                onClick = { if (username.isNotBlank()) onConfirm(username) },
                enabled = username.isNotBlank()
            ) {
                Text(t(lang, R.string.action_invite))
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(t(lang, R.string.action_cancel))
            }
        }
    )
}

