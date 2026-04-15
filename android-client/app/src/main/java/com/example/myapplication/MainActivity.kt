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
    }
}

// ---- Login / Register / Settings screen ----
@Composable
fun LoginScreen(state: UiState, vm: MessengerViewModel) {
    val context = androidx.compose.ui.platform.LocalContext.current
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
                "🔐 ${context.getString(R.string.app_title)}",
                fontSize = 24.sp,
                fontWeight = FontWeight.Bold,
                color = MaterialTheme.colorScheme.primary,
                modifier = Modifier.padding(bottom = 24.dp)
            )

            TabRow(selectedTabIndex = tab) {
                Tab(selected = tab == 0, onClick = { tab = 0 }, text = { Text(context.getString(R.string.login_tab_login)) })
                Tab(selected = tab == 1, onClick = { tab = 1 }, text = { Text(context.getString(R.string.login_tab_register)) })
                Tab(selected = tab == 2, onClick = { tab = 2 }, text = { Text(context.getString(R.string.login_tab_settings)) })
            }

            Spacer(Modifier.height(16.dp))

            when (tab) {
                0 -> {
                    OutlinedTextField(
                        value = loginUser, onValueChange = { loginUser = it },
                        label = { Text(context.getString(R.string.login_username)) }, singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next)
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = loginPass, onValueChange = { loginPass = it },
                        label = { Text(context.getString(R.string.login_password)) }, singleLine = true,
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
                        else Text(context.getString(R.string.login_button_login))
                    }
                }
                1 -> {
                    OutlinedTextField(
                        value = regUser, onValueChange = { regUser = it },
                        label = { Text(context.getString(R.string.login_username)) }, singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next)
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = regPass, onValueChange = { regPass = it },
                        label = { Text(context.getString(R.string.login_password)) }, singleLine = true,
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
                        else Text(context.getString(R.string.login_button_register))
                    }
                }
                2 -> {
                    OutlinedTextField(
                        value = cfgServer, onValueChange = { cfgServer = it },
                        label = { Text(context.getString(R.string.settings_server_address)) }, singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )
                    Spacer(Modifier.height(8.dp))
                    OutlinedTextField(
                        value = cfgProxy, onValueChange = { cfgProxy = it },
                        label = { Text(context.getString(R.string.settings_proxy_address)) }, singleLine = true,
                        modifier = Modifier.fillMaxWidth()
                    )
                    Spacer(Modifier.height(8.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Checkbox(checked = cfgDirect, onCheckedChange = { cfgDirect = it })
                        Text(context.getString(R.string.settings_direct_mode))
                    }
                    Spacer(Modifier.height(16.dp))
                    
                    // Language selection
                    LanguageSelector(context)
                    
                    Spacer(Modifier.height(16.dp))
                    Button(
                        onClick = { vm.saveConfig(AppConfig(cfgServer, cfgProxy, cfgDirect)) },
                        modifier = Modifier.fillMaxWidth()
                    ) { Text(context.getString(R.string.settings_button_save)) }

                    // Known federated servers list
                    if (state.knownServers.isNotEmpty()) {
                        Spacer(Modifier.height(16.dp))
                        Text(
                            context.getString(R.string.settings_known_servers),
                            fontSize = 13.sp,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        Spacer(Modifier.height(6.dp))
                        state.knownServers.forEach { addr ->
                            OutlinedButton(
                                onClick = {
                                    cfgServer = addr
                                    cfgDirect = true
                                    vm.saveConfig(AppConfig(addr, cfgProxy, true))
                                },
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(vertical = 2.dp)
                            ) {
                                Text(addr, fontSize = 13.sp, maxLines = 1)
                            }
                        }
                    }
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
    val context = androidx.compose.ui.platform.LocalContext.current
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
                title = { Text(context.getString(R.string.chat_title_general), fontWeight = FontWeight.Bold) },
                navigationIcon = {
                    IconButton(onClick = { showSidebar = !showSidebar }) {
                        Icon(Icons.Default.Person, contentDescription = context.getString(R.string.content_description_menu))
                    }
                },
                actions = {
                    IconButton(onClick = { vm.logout() }) {
                        Icon(Icons.AutoMirrored.Filled.ExitToApp, contentDescription = context.getString(R.string.content_description_logout))
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
                        placeholder = { Text(context.getString(R.string.chat_input_placeholder)) },
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
                        Icon(Icons.AutoMirrored.Filled.Send, contentDescription = context.getString(R.string.content_description_send),
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
    val context = androidx.compose.ui.platform.LocalContext.current
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
                        context.getString(R.string.sidebar_section_chats),
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                item {
                    NavigationItem(
                        text = context.getString(R.string.chat_title_general),
                        selected = state.screen == Screen.CHAT,
                        onClick = { vm.switchToGlobalChat(); onDismiss() }
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
                            context.getString(R.string.sidebar_section_dms),
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
                            context.getString(R.string.sidebar_section_rooms),
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
                        context.getString(R.string.sidebar_section_online, state.onlineUsers.size),
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
    val context = androidx.compose.ui.platform.LocalContext.current
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
                title = { Text("💬 ${state.currentDMPartner}", fontWeight = FontWeight.Bold) },
                navigationIcon = {
                    IconButton(onClick = { showSidebar = !showSidebar }) {
                        Icon(Icons.Default.Person, contentDescription = context.getString(R.string.content_description_menu))
                    }
                },
                actions = {
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
                        placeholder = { Text(context.getString(R.string.chat_input_placeholder_dm, state.currentDMPartner)) },
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
                        Icon(Icons.AutoMirrored.Filled.Send, contentDescription = context.getString(R.string.content_description_send),
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
    val context = androidx.compose.ui.platform.LocalContext.current
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
                Text(context.getString(R.string.chat_loading_room))
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
                            context.getString(R.string.room_members_count, room.members.size),
                            fontSize = 11.sp,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                },
                navigationIcon = {
                    IconButton(onClick = { showSidebar = !showSidebar }) {
                        Icon(Icons.Default.Person, contentDescription = context.getString(R.string.content_description_menu))
                    }
                },
                actions = {
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
                        placeholder = { Text(context.getString(R.string.chat_input_placeholder_room)) },
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
                        Icon(Icons.AutoMirrored.Filled.Send, contentDescription = context.getString(R.string.content_description_send),
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
fun NewDMDialog(onDismiss: () -> Unit, onConfirm: (String) -> Unit) {
    val context = androidx.compose.ui.platform.LocalContext.current
    var username by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(context.getString(R.string.dm_new_conversation)) },
        text = {
            OutlinedTextField(
                value = username,
                onValueChange = { username = it },
                label = { Text(context.getString(R.string.dm_target_user)) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )
        },
        confirmButton = {
            Button(
                onClick = { if (username.isNotBlank()) onConfirm(username) },
                enabled = username.isNotBlank()
            ) {
                Text(context.getString(R.string.dm_button_open))
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(context.getString(R.string.dm_button_cancel))
            }
        }
    )
}

@Composable
fun NewRoomDialog(onDismiss: () -> Unit, onConfirm: (String, Boolean, String) -> Unit) {
    val context = androidx.compose.ui.platform.LocalContext.current
    var name by remember { mutableStateOf("") }
    var description by remember { mutableStateOf("") }
    var isPublic by remember { mutableStateOf(false) }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(context.getString(R.string.room_create_title)) },
        text = {
            Column {
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    label = { Text(context.getString(R.string.room_name)) },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(Modifier.height(8.dp))
                OutlinedTextField(
                    value = description,
                    onValueChange = { description = it },
                    label = { Text(context.getString(R.string.room_description)) },
                    singleLine = true,
                    modifier = Modifier.fillMaxWidth()
                )
                Spacer(Modifier.height(8.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Checkbox(checked = isPublic, onCheckedChange = { isPublic = it })
                    Text(context.getString(R.string.room_public))
                }
            }
        },
        confirmButton = {
            Button(
                onClick = { if (name.isNotBlank()) onConfirm(name, isPublic, description) },
                enabled = name.isNotBlank()
            ) {
                Text(context.getString(R.string.room_button_create))
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(context.getString(R.string.room_button_cancel))
            }
        }
    )
}

@Composable
fun InviteToRoomDialog(onDismiss: () -> Unit, onConfirm: (String) -> Unit) {
    val context = androidx.compose.ui.platform.LocalContext.current
    var username by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(context.getString(R.string.room_invite_title)) },
        text = {
            OutlinedTextField(
                value = username,
                onValueChange = { username = it },
                label = { Text(context.getString(R.string.room_invite_username)) },
                singleLine = true,
                modifier = Modifier.fillMaxWidth()
            )
        },
        confirmButton = {
            Button(
                onClick = { if (username.isNotBlank()) onConfirm(username) },
                enabled = username.isNotBlank()
            ) {
                Text(context.getString(R.string.room_button_invite_ok))
            }
        },
        dismissButton = {
            TextButton(onClick = onDismiss) {
                Text(context.getString(R.string.room_button_cancel))
            }
        }
    )
}

// ---- Language Selector ----
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LanguageSelector(context: android.content.Context) {
    val supportedLanguages = LocaleManager.getSupportedLanguages()
    val currentLocale = LocaleManager.getCurrentLocale(context)
    var expanded by remember { mutableStateOf(false) }
    var selectedLanguage by remember { 
        mutableStateOf(
            supportedLanguages.find { it.code == currentLocale.language } 
                ?: supportedLanguages.first()
        )
    }

    Column(modifier = Modifier.fillMaxWidth()) {
        Text(
            text = context.getString(R.string.settings_language),
            fontSize = 14.sp,
            fontWeight = FontWeight.Medium,
            modifier = Modifier.padding(bottom = 4.dp)
        )
        
        ExposedDropdownMenuBox(
            expanded = expanded,
            onExpandedChange = { expanded = it }
        ) {
            OutlinedTextField(
                value = selectedLanguage.nativeName,
                onValueChange = {},
                readOnly = true,
                label = { Text(context.getString(R.string.settings_language_select)) },
                trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = expanded) },
                modifier = Modifier
                    .fillMaxWidth()
                    .menuAnchor(),
                colors = ExposedDropdownMenuDefaults.outlinedTextFieldColors()
            )
            
            ExposedDropdownMenu(
                expanded = expanded,
                onDismissRequest = { expanded = false }
            ) {
                supportedLanguages.forEach { language ->
                    DropdownMenuItem(
                        text = {
                            Column {
                                Text(
                                    text = language.nativeName,
                                    fontWeight = FontWeight.Medium
                                )
                                Text(
                                    text = language.displayName,
                                    fontSize = 12.sp,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        },
                        onClick = {
                            selectedLanguage = language
                            LocaleManager.setLocale(context, language.code)
                            expanded = false
                        },
                        contentPadding = ExposedDropdownMenuDefaults.ItemContentPadding
                    )
                }
            }
        }
    }
}
