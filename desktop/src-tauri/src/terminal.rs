use super::{backend, token};
use futures_util::{SinkExt, StreamExt};
use serde_json::{json, Value};
use std::{collections::HashMap, sync::Mutex, time::Duration};
use tauri::{ipc::Channel, State};
use tokio::sync::mpsc;
use tokio_tungstenite::{
    connect_async,
    tungstenite::{client::IntoClientRequest, Message},
};

#[derive(Default)]
pub struct TerminalTasks(Mutex<HashMap<String, TerminalTask>>);

struct TerminalTask {
    terminal_id: String,
    viewer_id: String,
    sender: mpsc::Sender<Message>,
    task: tauri::async_runtime::JoinHandle<()>,
}

#[tauri::command]
pub fn start_terminal(
    service_id: String,
    execution_id: String,
    viewer_id: String,
    after: u64,
    channel: Channel<Value>,
    state: State<TerminalTasks>,
) -> Result<(), String> {
    uuid::Uuid::parse_str(&service_id).map_err(|_| "Invalid service ID")?;
    uuid::Uuid::parse_str(&execution_id).map_err(|_| "Invalid execution ID")?;
    uuid::Uuid::parse_str(&viewer_id).map_err(|_| "Invalid viewer ID")?;
    start(
        format!("service:{service_id}"),
        execution_id.clone(),
        format!("/api/v1/services/{service_id}/terminal/{execution_id}"),
        viewer_id,
        after,
        channel,
        state,
    )
}

#[tauri::command]
pub fn start_shell_terminal(
    workspace_id: String,
    session_id: String,
    viewer_id: String,
    after: u64,
    channel: Channel<Value>,
    state: State<TerminalTasks>,
) -> Result<(), String> {
    uuid::Uuid::parse_str(&workspace_id).map_err(|_| "Invalid workspace ID")?;
    uuid::Uuid::parse_str(&session_id).map_err(|_| "Invalid terminal ID")?;
    uuid::Uuid::parse_str(&viewer_id).map_err(|_| "Invalid viewer ID")?;
    start(
        format!("shell:{session_id}"),
        session_id.clone(),
        format!("/api/v1/workspaces/{workspace_id}/terminals/{session_id}/stream"),
        viewer_id,
        after,
        channel,
        state,
    )
}

#[tauri::command]
pub fn start_docker_terminal(
    workspace_id: String,
    session_id: String,
    viewer_id: String,
    after: u64,
    channel: Channel<Value>,
    state: State<TerminalTasks>,
) -> Result<(), String> {
    uuid::Uuid::parse_str(&workspace_id).map_err(|_| "Invalid workspace ID")?;
    uuid::Uuid::parse_str(&session_id).map_err(|_| "Invalid Docker terminal ID")?;
    uuid::Uuid::parse_str(&viewer_id).map_err(|_| "Invalid viewer ID")?;
    start(
        format!("docker:{session_id}"),
        session_id.clone(),
        format!("/api/v1/workspaces/{workspace_id}/docker/terminals/{session_id}/stream"),
        viewer_id,
        after,
        channel,
        state,
    )
}

fn start(
    key: String,
    terminal_id: String,
    path: String,
    viewer_id: String,
    after: u64,
    channel: Channel<Value>,
    state: State<TerminalTasks>,
) -> Result<(), String> {
    let (sender, receiver) = mpsc::channel(128);
    let task = tauri::async_runtime::spawn(run_terminal(path, after, channel, receiver));
    let mut tasks = state.0.lock().map_err(|_| "Terminal state unavailable")?;
    if let Some(previous) = tasks.insert(
        key,
        TerminalTask {
            terminal_id,
            viewer_id,
            sender,
            task,
        },
    ) {
        previous.task.abort();
    }
    Ok(())
}

#[tauri::command]
pub fn stop_terminal(
    service_id: String,
    execution_id: String,
    viewer_id: String,
    state: State<TerminalTasks>,
) {
    stop(
        format!("service:{service_id}"),
        execution_id,
        viewer_id,
        state,
    )
}

#[tauri::command]
pub fn stop_shell_terminal(session_id: String, viewer_id: String, state: State<TerminalTasks>) {
    stop(format!("shell:{session_id}"), session_id, viewer_id, state)
}

#[tauri::command]
pub fn stop_docker_terminal(session_id: String, viewer_id: String, state: State<TerminalTasks>) {
    stop(format!("docker:{session_id}"), session_id, viewer_id, state)
}

fn stop(key: String, terminal_id: String, viewer_id: String, state: State<TerminalTasks>) {
    if let Ok(mut tasks) = state.0.lock() {
        if tasks
            .get(&key)
            .is_some_and(|task| task.terminal_id == terminal_id && task.viewer_id == viewer_id)
        {
            if let Some(task) = tasks.remove(&key) {
                task.task.abort();
            }
        }
    }
}

fn send(
    state: &State<TerminalTasks>,
    key: &str,
    terminal_id: &str,
    message: Value,
) -> Result<(), String> {
    let tasks = state.0.lock().map_err(|_| "Terminal state unavailable")?;
    let task = tasks
        .get(key)
        .filter(|task| task.terminal_id == terminal_id)
        .ok_or("Terminal connection unavailable")?;
    task.sender
        .try_send(Message::Text(message.to_string().into()))
        .map_err(|_| "Terminal connection busy".into())
}

#[tauri::command]
pub fn terminal_input(
    service_id: String,
    execution_id: String,
    data: String,
    state: State<TerminalTasks>,
) -> Result<(), String> {
    if data.is_empty() || data.len() > 4096 {
        return Err("Terminal input must be 1–4096 bytes".into());
    }
    send(
        &state,
        &format!("service:{service_id}"),
        &execution_id,
        json!({"type":"input", "data":data}),
    )
}

#[tauri::command]
pub fn terminal_resize(
    service_id: String,
    execution_id: String,
    cols: u16,
    rows: u16,
    state: State<TerminalTasks>,
) -> Result<(), String> {
    if cols == 0 || rows == 0 || cols > 500 || rows > 300 {
        return Err("Invalid terminal size".into());
    }
    send(
        &state,
        &format!("service:{service_id}"),
        &execution_id,
        json!({"type":"resize", "cols":cols, "rows":rows}),
    )
}

#[tauri::command]
pub fn shell_terminal_input(
    session_id: String,
    data: String,
    state: State<TerminalTasks>,
) -> Result<(), String> {
    if data.is_empty() || data.len() > 4096 {
        return Err("Terminal input must be 1–4096 bytes".into());
    }
    send(
        &state,
        &format!("shell:{session_id}"),
        &session_id,
        json!({"type":"input", "data":data}),
    )
}

#[tauri::command]
pub fn docker_terminal_input(
    session_id: String,
    data: String,
    state: State<TerminalTasks>,
) -> Result<(), String> {
    if data.is_empty() || data.len() > 4096 {
        return Err("Terminal input must be 1–4096 bytes".into());
    }
    send(
        &state,
        &format!("docker:{session_id}"),
        &session_id,
        json!({"type":"input", "data":data}),
    )
}

#[tauri::command]
pub fn shell_terminal_resize(
    session_id: String,
    cols: u16,
    rows: u16,
    state: State<TerminalTasks>,
) -> Result<(), String> {
    if cols == 0 || rows == 0 || cols > 500 || rows > 300 {
        return Err("Invalid terminal size".into());
    }
    send(
        &state,
        &format!("shell:{session_id}"),
        &session_id,
        json!({"type":"resize", "cols":cols, "rows":rows}),
    )
}

#[tauri::command]
pub fn docker_terminal_resize(
    session_id: String,
    cols: u16,
    rows: u16,
    state: State<TerminalTasks>,
) -> Result<(), String> {
    if cols == 0 || rows == 0 || cols > 500 || rows > 300 {
        return Err("Invalid terminal size".into());
    }
    send(
        &state,
        &format!("docker:{session_id}"),
        &session_id,
        json!({"type":"resize", "cols":cols, "rows":rows}),
    )
}

async fn run_terminal(
    path: String,
    mut after: u64,
    channel: Channel<Value>,
    mut receiver: mpsc::Receiver<Message>,
) {
    loop {
        let _ = channel.send(json!({"type":"reconnecting"}));
        let connection = async {
            let (base, token_path) = backend()?;
            let url = format!(
                "{}{path}?after={after}",
                base.replacen("http://", "ws://", 1)
            );
            let mut request = url
                .into_client_request()
                .map_err(|_| "Cannot create terminal request".to_string())?;
            request.headers_mut().insert(
                "Authorization",
                format!("Bearer {}", token(&token_path)?)
                    .parse()
                    .map_err(|_| "Invalid token header")?,
            );
            connect_async(request).await.map_err(|error| match error {
                tokio_tungstenite::tungstenite::Error::Http(response)
                    if matches!(response.status().as_u16(), 404 | 409) =>
                {
                    "stopped".to_string()
                }
                tokio_tungstenite::tungstenite::Error::Http(response)
                    if matches!(response.status().as_u16(), 401 | 403) =>
                {
                    "authentication-error".to_string()
                }
                _ => "Terminal connection unavailable".to_string(),
            })
        }
        .await;
        match connection {
            Ok((socket, _)) => {
                let _ = channel.send(json!({"type":"connected"}));
                let (mut writer, mut reader) = socket.split();
                loop {
                    tokio::select! {
                        incoming = reader.next() => match incoming {
                            Some(Ok(Message::Text(text))) => {
                                if let Ok(frame) = serde_json::from_str::<Value>(&text) {
                                    if let Some(sequence) = frame["sequence"].as_u64() { after = sequence; }
                                    let stopped = frame["type"] == "stopped";
                                    let _ = channel.send(frame);
                                    if stopped { return; }
                                }
                            }
                            Some(Ok(Message::Close(_))) | None | Some(Err(_)) => break,
                            _ => {}
                        },
                        outgoing = receiver.recv() => match outgoing {
                            Some(message) => { if writer.send(message).await.is_err() { break; } }
                            None => return,
                        }
                    }
                }
            }
            Err(reason) if reason == "stopped" || reason == "authentication-error" => {
                let _ = channel.send(json!({"type":reason}));
                return;
            }
            Err(_) => {}
        }
        tokio::time::sleep(Duration::from_secs(2)).await;
    }
}
