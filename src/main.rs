use axum::{
    Json, Router,
    extract::{
        Path, Query, State,
        ws::{Message, WebSocket, WebSocketUpgrade},
    },
    response::IntoResponse,
    routing::{get, post},
};
use futures::{sink::SinkExt, stream::StreamExt};
use serde::{Deserialize, Serialize};
use std::{
    collections::HashMap,
    sync::{Arc, Mutex},
};
use tokio::sync::broadcast;
use uuid::Uuid;

// Maximum number of messages that can be held in the channel
const MAX_CHANNEL_CAPACITY: usize = 100;

// Message structure to identify the sender
#[derive(Debug, Clone)]
struct ChannelMessage {
    sender_id: String,
    content: String,
}

// Channel information
#[derive(Debug, Clone)]
struct Channel {
    id: String,
    password: String,
    tx: broadcast::Sender<ChannelMessage>,
}

// App state shared across connections
#[derive(Clone)]
struct AppState {
    channels: Arc<Mutex<HashMap<String, Channel>>>,
}

// Request body for creating a channel
#[derive(Deserialize)]
struct CreateChannelRequest {
    password: String,
}

// Response for channel creation
#[derive(Serialize)]
struct CreateChannelResponse {
    channel_id: String,
}

// Query parameters for joining a channel
#[derive(Deserialize)]
struct JoinChannelQuery {
    password: String,
}

#[tokio::main]
async fn main() {
    // Initialize state
    let app_state = AppState {
        channels: Arc::new(Mutex::new(HashMap::new())),
    };

    // Create router
    let app = Router::new()
        .route("/channel", post(create_channel))
        .route("/channel/{channel_id}", get(join_channel))
        .with_state(app_state);

    // Run the server
    let listener = tokio::net::TcpListener::bind("127.0.0.1:3000")
        .await
        .unwrap();
    println!("Server running on http://127.0.0.1:3000");
    axum::serve(listener, app).await.unwrap();
}

// Handler for creating a new channel
async fn create_channel(
    State(state): State<AppState>,
    Json(payload): Json<CreateChannelRequest>,
) -> impl IntoResponse {
    let channel_id = Uuid::new_v4().to_string();
    let (tx, _rx) = broadcast::channel(MAX_CHANNEL_CAPACITY);

    let channel = Channel {
        id: channel_id.clone(),
        password: payload.password,
        tx,
    };

    // Store the channel
    state
        .channels
        .lock()
        .unwrap()
        .insert(channel_id.clone(), channel);

    println!("Created channel with ID: {}", channel_id);
    Json(CreateChannelResponse {
        channel_id: channel_id,
    })
}

// Handler for joining an existing channel
async fn join_channel(
    ws: WebSocketUpgrade,
    Path(channel_id): Path<String>,
    Query(params): Query<JoinChannelQuery>,
    State(state): State<AppState>,
) -> impl IntoResponse {
    // Check if channel exists and password matches
    let channel_opt = {
        let channels = state.channels.lock().unwrap();
        channels.get(&channel_id).cloned()
    };

    match channel_opt {
        Some(channel) if channel.password == params.password => {
            // Password matches, upgrade the connection to WebSocket
            println!("User joining channel: {}", channel.id);
            ws.on_upgrade(move |socket| handle_socket(socket, channel))
        }
        _ => {
            // Channel not found or password doesn't match
            println!("Failed join attempt for channel: {}", channel_id);
            ws.on_upgrade(|socket| handle_invalid_connection(socket))
        }
    }
}

// Handle invalid connection attempts
async fn handle_invalid_connection(mut socket: WebSocket) {
    // Send error message
    let _ = socket
        .send(Message::Text(
            "Invalid channel ID or password".to_string().into(),
        ))
        .await;
    // Close connection
    let _ = socket.close().await;
}

// Handle valid WebSocket connections
async fn handle_socket(socket: WebSocket, channel: Channel) {
    // Generate a unique ID for this connection
    let connection_id = Uuid::new_v4().to_string();
    println!(
        "New connection {} established in channel: {}",
        connection_id, channel.id
    );

    // Split socket into sender and receiver
    let (mut sender, mut receiver) = socket.split();

    // Subscribe to the broadcast channel
    let mut rx = channel.tx.subscribe();
    let channel_id = channel.id.clone();

    // Forward messages from channel to this WebSocket, except own messages
    let connection_id_clone = connection_id.clone();
    let mut send_task = tokio::spawn(async move {
        while let Ok(msg) = rx.recv().await {
            // Don't send message back to the sender that originated it
            if msg.sender_id != connection_id_clone {
                if sender
                    .send(Message::Text(msg.content.into()))
                    .await
                    .is_err()
                {
                    println!(
                        "Failed to send message to WebSocket in channel: {}",
                        channel_id
                    );
                    break;
                }
            }
        }
    });

    // Forward messages from WebSocket to channel
    let tx = channel.tx.clone();
    let channel_id = channel.id;
    let mut recv_task = tokio::spawn(async move {
        while let Some(Ok(Message::Text(text))) = receiver.next().await {
            // Convert Utf8Bytes to String
            let text_string = text.to_string();

            // Create message with sender ID
            let channel_message = ChannelMessage {
                sender_id: connection_id.clone(),
                content: text_string,
            };

            // Broadcast the message to all subscribers
            println!(
                "Broadcasting message from {} in channel: {}",
                connection_id, channel_id
            );
            let _ = tx.send(channel_message);
        }
        println!(
            "WebSocket connection {} closed in channel: {}",
            connection_id, channel_id
        );
    });

    // Wait for either task to finish
    tokio::select! {
        _ = (&mut send_task) => {
            recv_task.abort();
        }
        _ = (&mut recv_task) => {
            send_task.abort();
        }
    }
}
