#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use futures_util::StreamExt;
use serde::Serialize;
use serde_json::Value;
use std::{
    collections::{BTreeSet, HashMap},
    env, fs,
    net::IpAddr,
    path::PathBuf,
    process::Command,
    sync::Mutex,
    time::Duration,
};
use tauri::{ipc::Channel, Emitter, State};

mod terminal;

#[derive(Default)]
struct StreamTask(Mutex<Option<tauri::async_runtime::JoinHandle<()>>>);

#[derive(Default)]
struct DockerLogTasks(Mutex<HashMap<String, tauri::async_runtime::JoinHandle<()>>>);

#[derive(Serialize)]
struct Reply {
    status: u16,
    body: Value,
}

fn backend() -> Result<(String, PathBuf), String> {
    let host = env::var("DEVORBIT_HOST").unwrap_or_else(|_| "127.0.0.1".into());
    let ip: IpAddr = host
        .parse()
        .map_err(|_| "DEVORBIT_HOST must be a loopback IP")?;
    if !ip.is_loopback() {
        return Err("DEVORBIT_HOST must be a loopback IP".into());
    }
    let port: u16 = env::var("DEVORBIT_PORT")
        .unwrap_or_else(|_| "7456".into())
        .parse()
        .map_err(|_| "DEVORBIT_PORT must be between 1 and 65535")?;
    if port == 0 {
        return Err("DEVORBIT_PORT must be between 1 and 65535".into());
    }
    let data_dir = match env::var("DEVORBIT_DATA_DIR") {
        Ok(path) if !path.is_empty() => PathBuf::from(path),
        _ => dirs::config_dir()
            .ok_or("Cannot locate user config directory")?
            .join("devorbit"),
    };
    let token_path = match env::var("DEVORBIT_AUTH_TOKEN_PATH") {
        Ok(path) if !path.is_empty() => PathBuf::from(path),
        _ => data_dir.join("auth-token"),
    };
    if !data_dir.is_absolute() || !token_path.is_absolute() {
        return Err("DevOrbit data and token paths must be absolute".into());
    }
    Ok((
        format!("http://{}", std::net::SocketAddr::new(ip, port)),
        token_path,
    ))
}

fn token(path: &PathBuf) -> Result<String, String> {
    let token = fs::read_to_string(path)
        .map_err(|_| "Cannot read backend token. Start the Go backend first.".to_string())?;
    if token.len() != 64 || !token.bytes().all(|b| b.is_ascii_hexdigit()) {
        return Err("Backend token has an invalid format".into());
    }
    Ok(token)
}

fn font_families(output: &str) -> Vec<String> {
    output
        .lines()
        .flat_map(|line| line.split(','))
        .map(str::trim)
        .filter(|family| !family.is_empty())
        .map(str::to_owned)
        .collect::<BTreeSet<_>>()
        .into_iter()
        .collect()
}

#[tauri::command]
fn list_system_fonts() -> Result<Vec<String>, String> {
    let output = Command::new("fc-list")
        .args(["-f", "%{family}\n"])
        .output()
        .map_err(|_| "Cannot list system fonts: fontconfig is unavailable".to_string())?;
    if !output.status.success() {
        return Err("Cannot list system fonts".into());
    }
    Ok(font_families(&String::from_utf8_lossy(&output.stdout)))
}

fn allowed(method: &str, path: &str) -> bool {
    let (route, query) = path.split_once('?').unwrap_or((path, ""));
    let parts: Vec<_> = route.split('/').collect();
    let id = |segment: &str| uuid::Uuid::parse_str(segment).is_ok();
    let valid = match parts.as_slice() {
        ["", "api", "v1", "workspaces"] => matches!(method, "GET" | "POST"),
        ["", "api", "v1", "workspaces", wid] if id(wid) => matches!(method, "PATCH" | "DELETE"),
        ["", "api", "v1", "workspaces", wid, "services"] if id(wid) => {
            matches!(method, "GET" | "POST")
        }
        ["", "api", "v1", "workspaces", wid, "configuration"] if id(wid) => {
            matches!(method, "GET" | "PUT")
        }
        ["", "api", "v1", "workspaces", wid, "configuration", "validate"] if id(wid) => {
            method == "POST"
        }
        ["", "api", "v1", "workspaces", wid, action]
            if id(wid) && matches!(*action, "start" | "stop") =>
        {
            method == "POST"
        }
        ["", "api", "v1", "workspaces", wid, "status"] if id(wid) => method == "GET",
        ["", "api", "v1", "workspaces", wid, "docker"] if id(wid) => method == "GET",
        ["", "api", "v1", "workspaces", wid, "traffic"] if id(wid) => {
            matches!(method, "GET" | "POST")
        }
        ["", "api", "v1", "workspaces", wid, "traffic", sid] if id(wid) && id(sid) => {
            matches!(method, "GET" | "DELETE")
        }
        ["", "api", "v1", "workspaces", wid, "traffic", sid, "records"] if id(wid) && id(sid) => {
            matches!(method, "GET" | "DELETE")
        }
        ["", "api", "v1", "workspaces", wid, "docker", "operations"] if id(wid) => method == "POST",
        ["", "api", "v1", "workspaces", wid, "docker", "operations", oid] if id(wid) && id(oid) => {
            matches!(method, "GET" | "DELETE")
        }
        ["", "api", "v1", "workspaces", wid, "docker", "terminals"] if id(wid) => method == "POST",
        ["", "api", "v1", "workspaces", wid, "docker", "terminals", tid] if id(wid) && id(tid) => {
            method == "DELETE"
        }
        ["", "api", "v1", "workspaces", wid, "terminals"] if id(wid) => {
            matches!(method, "GET" | "POST")
        }
        ["", "api", "v1", "workspaces", wid, "terminals", tid] if id(wid) && id(tid) => {
            method == "DELETE"
        }
        ["", "api", "v1", "services", sid] if id(sid) => {
            matches!(method, "GET" | "PATCH" | "DELETE")
        }
        ["", "api", "v1", "services", sid, action]
            if id(sid) && matches!(*action, "start" | "stop" | "restart") =>
        {
            method == "POST"
        }
        ["", "api", "v1", "services", sid, "logs"] if id(sid) => method == "GET",
        ["", "api", "v1", "services", sid, "environment"] if id(sid) => method == "GET",
        ["", "api", "v1", "services", sid, "environment", name]
            if id(sid) && (*name == ".env" || name.starts_with(".env.") && name.len() > 5) =>
        {
            matches!(method, "GET" | "PUT")
        }
        _ => false,
    };
    let env_file_route = matches!(
        parts.as_slice(),
        ["", "api", "v1", "services", _, "environment", _]
    );
    if !valid || path.contains('#') || path.contains('\\') || route.contains('%') && !env_file_route
    {
        return false;
    }
    if query.is_empty() {
        return true;
    }
    if method != "GET"
        && !(method == "DELETE" && matches!(parts.as_slice(), ["", "api", "v1", "services", _]))
    {
        return false;
    }
    let parsed = match reqwest::Url::parse(&format!("http://127.0.0.1{path}")) {
        Ok(url) => url,
        Err(_) => return false,
    };
    parsed.query_pairs().all(|(key, _)| match parts.as_slice() {
        ["", "api", "v1", "workspaces"] | ["", "api", "v1", "workspaces", _, "services"] => {
            matches!(key.as_ref(), "limit" | "offset")
        }
        ["", "api", "v1", "services", _, "logs"] => matches!(key.as_ref(), "limit" | "before"),
        ["", "api", "v1", "services", _] => key == "revision",
        ["", "api", "v1", "workspaces", _, "docker"] => key == "file",
        ["", "api", "v1", "workspaces", _, "traffic", _, "records"] => {
            matches!(key.as_ref(), "after" | "limit")
        }
        _ => false,
    })
}

#[tauri::command]
async fn api_request(method: String, path: String, body: Option<Value>) -> Result<Reply, String> {
    if !allowed(&method, &path) {
        return Err("Unsupported DevOrbit API request".into());
    }
    if body.is_some() && !matches!(method.as_str(), "POST" | "PATCH" | "PUT") {
        return Err("Request body is not allowed for this operation".into());
    }
    let (base, token_path) = backend()?;
    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(30))
        .no_proxy()
        .redirect(reqwest::redirect::Policy::none())
        .build()
        .map_err(|_| "Cannot initialize API client")?;
    let mut request = client
        .request(
            method.parse().map_err(|_| "Invalid HTTP method")?,
            format!("{base}{path}"),
        )
        .bearer_auth(token(&token_path)?);
    if let Some(body) = body {
        request = request.json(&body);
    }
    let response = request
        .send()
        .await
        .map_err(|_| "Backend unavailable. Check that the Go server is running.".to_string())?;
    let status = response.status().as_u16();
    let body = if status == 204 {
        Value::Null
    } else {
        response
            .json::<Value>()
            .await
            .map_err(|_| "Invalid JSON response from backend")?
    };
    Ok(Reply { status, body })
}

#[tauri::command]
fn start_docker_logs(
    workspace_id: String,
    file: String,
    service: String,
    tail: u16,
    viewer_id: String,
    channel: Channel<Value>,
    state: State<DockerLogTasks>,
) -> Result<(), String> {
    uuid::Uuid::parse_str(&workspace_id).map_err(|_| "Invalid workspace ID")?;
    uuid::Uuid::parse_str(&viewer_id).map_err(|_| "Invalid viewer ID")?;
    if tail == 0 || tail > 2000 || service.is_empty() || service.len() > 255 {
        return Err("Invalid Docker log request".into());
    }
    let task = tauri::async_runtime::spawn(async move {
        let result: Result<(), String> = async {
            let (base, token_path) = backend()?;
            let mut url = reqwest::Url::parse(&format!(
                "{base}/api/v1/workspaces/{workspace_id}/docker/services/"
            ))
            .map_err(|_| "Invalid Docker log URL".to_string())?;
            url.path_segments_mut()
                .map_err(|_| "Invalid Docker log path")?
                .pop_if_empty()
                .push(&service)
                .push("logs");
            url.query_pairs_mut()
                .append_pair("file", &file)
                .append_pair("tail", &tail.to_string());
            let client = reqwest::Client::builder()
                .no_proxy()
                .redirect(reqwest::redirect::Policy::none())
                .build()
                .map_err(|_| "Cannot initialize Docker log client".to_string())?;
            let response = client
                .get(url)
                .bearer_auth(token(&token_path)?)
                .send()
                .await
                .map_err(|_| "Docker log connection unavailable".to_string())?;
            if !response.status().is_success() {
                let status = response.status();
                let body = response.json::<Value>().await.unwrap_or(Value::Null);
                return Err(body["error"]["message"]
                    .as_str()
                    .map(str::to_owned)
                    .unwrap_or_else(|| format!("Docker logs failed ({status})")));
            }
            let _ = channel.send(serde_json::json!({"stream":"status","data":"connected"}));
            let mut chunks = response.bytes_stream();
            let mut pending = Vec::new();
            while let Some(chunk) = chunks.next().await {
                let bytes = chunk.map_err(|_| "Docker log stream disconnected".to_string())?;
                pending.extend_from_slice(&bytes);
                while let Some(end) = pending.windows(2).position(|pair| pair == b"\n\n") {
                    let frame: Vec<_> = pending.drain(..end + 2).collect();
                    if let Ok(frame) = std::str::from_utf8(&frame) {
                        if let Some(value) =
                            frame.lines().find_map(|line| line.strip_prefix("data: "))
                        {
                            if let Ok(value) = serde_json::from_str::<Value>(value) {
                                let _ = channel.send(value);
                            }
                        }
                    }
                }
                if pending.len() > 1_048_576 {
                    return Err("Docker log frame exceeded 1 MiB".into());
                }
            }
            Err("Docker log stream ended. Refresh to reconnect.".to_string())
        }
        .await;
        if let Err(problem) = result {
            let _ = channel.send(serde_json::json!({"stream":"error","data":problem}));
        }
    });
    let mut tasks = state.0.lock().map_err(|_| "Docker log state unavailable")?;
    if let Some(previous) = tasks.insert(viewer_id, task) {
        previous.abort();
    }
    Ok(())
}

#[tauri::command]
fn stop_docker_logs(viewer_id: String, state: State<DockerLogTasks>) {
    if let Ok(mut tasks) = state.0.lock() {
        if let Some(task) = tasks.remove(&viewer_id) {
            task.abort();
        }
    }
}

async fn stream(app: tauri::AppHandle) {
    loop {
        let connection = async {
            let (base, token_path) = backend()?;
            let client = reqwest::Client::builder()
                .no_proxy()
                .redirect(reqwest::redirect::Policy::none())
                .build()
                .map_err(|_| "Cannot initialize event client".to_string())?;
            client
                .get(format!("{base}/api/v1/events"))
                .bearer_auth(token(&token_path)?)
                .send()
                .await
                .map_err(|_| "backend unavailable".to_string())
        }
        .await;
        let mut auth_error = false;
        match connection {
            Ok(response) if response.status().is_success() => {
                let _ = app.emit("stream-state", "connected");
                let mut chunks = response.bytes_stream();
                let mut pending = Vec::new();
                while let Some(Ok(bytes)) = chunks.next().await {
                    pending.extend_from_slice(&bytes);
                    while let Some(end) = pending.windows(2).position(|bytes| bytes == b"\n\n") {
                        let frame: Vec<_> = pending.drain(..end + 2).collect();
                        if let Ok(frame) = std::str::from_utf8(&frame) {
                            for line in frame.lines() {
                                if let Some(data) = line.strip_prefix("data: ") {
                                    if let Ok(value) = serde_json::from_str::<Value>(data) {
                                        if value["workspace_id"].as_str().is_some() {
                                            let _ = app.emit("backend-event", value);
                                        }
                                    }
                                }
                            }
                        }
                    }
                    if pending.len() > 1_048_576 {
                        break;
                    }
                }
            }
            Ok(response)
                if response.status().as_u16() == 401 || response.status().as_u16() == 403 =>
            {
                auth_error = true;
                let _ = app.emit("stream-state", "authentication-error");
            }
            _ => {}
        }
        if !auth_error {
            let _ = app.emit("stream-state", "reconnecting");
        }
        tokio::time::sleep(Duration::from_secs(2)).await;
    }
}

#[tauri::command]
fn start_events(app: tauri::AppHandle, state: State<StreamTask>) {
    let mut task = state.0.lock().expect("stream task lock");
    if task
        .as_ref()
        .is_some_and(|task| !task.inner().is_finished())
    {
        return;
    }
    *task = Some(tauri::async_runtime::spawn(stream(app)));
}

#[tauri::command]
fn stop_events(state: State<StreamTask>) {
    if let Some(task) = state.0.lock().expect("stream task lock").take() {
        task.abort();
    }
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .manage(StreamTask::default())
        .manage(terminal::TerminalTasks::default())
        .manage(DockerLogTasks::default())
        .invoke_handler(tauri::generate_handler![
            api_request,
            list_system_fonts,
            start_events,
            stop_events,
            terminal::start_terminal,
            terminal::stop_terminal,
            terminal::terminal_input,
            terminal::terminal_resize,
            terminal::start_shell_terminal,
            terminal::stop_shell_terminal,
            terminal::shell_terminal_input,
            terminal::shell_terminal_resize,
            terminal::start_docker_terminal,
            terminal::stop_docker_terminal,
            terminal::docker_terminal_input,
            terminal::docker_terminal_resize,
            start_docker_logs,
            stop_docker_logs
        ])
        .run(tauri::generate_context!())
        .expect("error while running DevOrbit");
}

#[cfg(test)]
mod tests {
    use super::{allowed, font_families};

    #[test]
    fn font_list_is_unique_and_includes_family_aliases() {
        assert_eq!(
            font_families("Mono,Mono Alt\nSans\nMono\n"),
            ["Mono", "Mono Alt", "Sans"]
        );
    }

    #[test]
    fn bridge_only_allows_known_routes() {
        let id = "30c80606-7a89-4b84-94dd-084e33e88baa";
        assert!(allowed(
            "GET",
            &format!("/api/v1/services/{id}/logs?limit=100")
        ));
        assert!(allowed(
            "GET",
            &format!("/api/v1/services/{id}/environment")
        ));
        assert!(allowed(
            "GET",
            &format!("/api/v1/services/{id}/environment/.env")
        ));
        assert!(allowed(
            "PUT",
            &format!("/api/v1/services/{id}/environment/.env.local")
        ));
        assert!(allowed(
            "GET",
            &format!("/api/v1/services/{id}/environment/.env.local%20example")
        ));
        assert!(!allowed(
            "DELETE",
            &format!("/api/v1/services/{id}/environment/.env")
        ));
        assert!(!allowed(
            "GET",
            &format!("/api/v1/services/{id}/environment/.env?x=1")
        ));
        assert!(!allowed(
            "GET",
            &format!("/api/v1/services/{id}/environment/../logs")
        ));
        assert!(allowed("POST", &format!("/api/v1/services/{id}/restart")));
        assert!(allowed(
            "GET",
            &format!("/api/v1/workspaces/{id}/docker?file=compose.yaml")
        ));
        assert!(allowed(
            "POST",
            &format!("/api/v1/workspaces/{id}/docker/operations")
        ));
        assert!(allowed(
            "GET",
            &format!("/api/v1/workspaces/{id}/docker/operations/{id}")
        ));
        assert!(!allowed(
            "POST",
            &format!("/api/v1/workspaces/{id}/docker/evil")
        ));
        assert!(allowed(
            "GET",
            &format!("/api/v1/workspaces/{id}/terminals")
        ));
        assert!(allowed(
            "POST",
            &format!("/api/v1/workspaces/{id}/terminals")
        ));
        assert!(allowed(
            "DELETE",
            &format!("/api/v1/workspaces/{id}/terminals/{id}")
        ));
        assert!(!allowed(
            "GET",
            &format!("/api/v1/workspaces/{id}/terminals/{id}/stream")
        ));
        assert!(allowed("POST", &format!("/api/v1/workspaces/{id}/traffic")));
        assert!(allowed("GET", &format!("/api/v1/workspaces/{id}/traffic")));
        assert!(allowed(
            "DELETE",
            &format!("/api/v1/workspaces/{id}/traffic/{id}")
        ));
        assert!(allowed(
            "GET",
            &format!("/api/v1/workspaces/{id}/traffic/{id}/records?after=1&limit=50")
        ));
        assert!(!allowed(
            "POST",
            &format!("/api/v1/workspaces/{id}/traffic/{id}/records")
        ));
        assert!(!allowed(
            "GET",
            &format!("/api/v1/workspaces/{id}/traffic/{id}/records?upstream=http://example.com")
        ));
        assert!(!allowed("GET", "http://example.com"));
        assert!(!allowed("DELETE", "/api/v1/workspaces"));
        assert!(!allowed("GET", "/api/v1/events"));
        assert!(!allowed(
            "GET",
            &format!("/api/v1/services/{id}/logs?redirect=http://example.com")
        ));
    }
}
