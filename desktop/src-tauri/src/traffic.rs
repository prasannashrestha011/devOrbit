use std::{
    fs,
    io::Write,
    path::Path,
    time::{Duration, SystemTime, UNIX_EPOCH},
};
use tauri_plugin_dialog::DialogExt;

fn body_path(
    workspace_id: &str,
    session_id: &str,
    record_id: &str,
    side: &str,
) -> Result<String, String> {
    for id in [workspace_id, session_id, record_id] {
        uuid::Uuid::parse_str(id).map_err(|_| "Invalid capture ID")?;
    }
    if !matches!(side, "request" | "response") {
        return Err("Invalid capture body".into());
    }
    Ok(format!(
        "/api/v1/workspaces/{workspace_id}/traffic/{session_id}/records/{record_id}/body/{side}"
    ))
}

async fn save_response(mut response: reqwest::Response, destination: &Path) -> Result<(), String> {
    if !response.status().is_success() {
        return Err("Captured body is unavailable".into());
    }
    let parent = destination.parent().ok_or("Invalid destination")?;
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|_| "Invalid system clock")?
        .as_nanos();
    let temporary = parent.join(format!(
        ".devorbit-traffic-{}-{nonce}.part",
        std::process::id()
    ));
    let mut options = fs::OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    let mut file = options
        .open(&temporary)
        .map_err(|_| "Cannot create download file")?;
    let result = async {
        while let Some(bytes) = response
            .chunk()
            .await
            .map_err(|_| "Body download was interrupted")?
        {
            file.write_all(&bytes)
                .map_err(|_| "Cannot write downloaded body")?;
        }
        file.sync_all()
            .map_err(|_| "Cannot finish downloaded body")?;
        Ok::<(), &str>(())
    }
    .await;
    drop(file);
    let result = result.and_then(|()| {
        fs::rename(&temporary, destination)
            .map_err(|_| "Cannot save body; existing destination was left unchanged")
    });
    if result.is_err() {
        let _ = fs::remove_file(&temporary);
    }
    result.map_err(str::to_owned)
}

#[tauri::command]
pub async fn download_traffic_body(
    app: tauri::AppHandle,
    workspace_id: String,
    session_id: String,
    record_id: String,
    side: String,
) -> Result<Option<String>, String> {
    let path = body_path(&workspace_id, &session_id, &record_id, &side)?;
    let filename = format!("{side}-{record_id}.bin");
    let selected = tauri::async_runtime::spawn_blocking(move || {
        app.dialog()
            .file()
            .set_file_name(filename)
            .blocking_save_file()
    })
    .await
    .map_err(|_| "Could not open save dialog")?;
    let Some(selected) = selected else {
        return Ok(None);
    };
    let destination = selected
        .into_path()
        .map_err(|_| "Choose a local destination")?;
    let (base, token_path) = super::backend()?;
    let response = reqwest::Client::builder()
        .no_proxy()
        .redirect(reqwest::redirect::Policy::none())
        .timeout(Duration::from_secs(300))
        .build()
        .map_err(|_| "Cannot initialize download client")?
        .get(format!("{base}{path}"))
        .bearer_auth(super::token(&token_path)?)
        .send()
        .await
        .map_err(|_| "Backend unavailable")?;
    save_response(response, &destination).await?;
    Ok(Some(destination.to_string_lossy().into_owned()))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::{io::Read, net::TcpListener};

    #[test]
    fn download_paths_require_ids_and_a_known_body_side() {
        let id = "30c80606-7a89-4b84-94dd-084e33e88baa";
        assert!(body_path(id, id, id, "request").is_ok());
        assert!(body_path(id, id, "../secret", "response").is_err());
        assert!(body_path(id, id, id, "../../secret").is_err());
    }

    #[tokio::test]
    async fn downloads_preserve_bytes_and_leave_existing_files_on_network_failure() {
        let nonce = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let root = std::env::temp_dir().join(format!("devorbit-download-test-{nonce}"));
        fs::create_dir(&root).unwrap();
        let destination = root.join("body.bin");
        for (length, succeeds) in [(4, true), (10, false)] {
            fs::write(&destination, b"original").unwrap();
            let listener = TcpListener::bind("127.0.0.1:0").unwrap();
            let address = listener.local_addr().unwrap();
            let server = std::thread::spawn(move || {
                let (mut stream, _) = listener.accept().unwrap();
                let mut request = [0; 4096];
                let _ = stream.read(&mut request);
                write!(
                    stream,
                    "HTTP/1.1 200 OK\r\nContent-Length: {length}\r\nConnection: close\r\n\r\n"
                )
                .unwrap();
                stream.write_all(&[0, 255, 10, 42]).unwrap();
            });
            let response = reqwest::Client::builder()
                .no_proxy()
                .build()
                .unwrap()
                .get(format!("http://{address}"))
                .send()
                .await
                .unwrap();
            assert_eq!(
                save_response(response, &destination).await.is_ok(),
                succeeds
            );
            assert_eq!(
                fs::read(&destination).unwrap(),
                if succeeds {
                    vec![0, 255, 10, 42]
                } else {
                    b"original".to_vec()
                }
            );
            assert_eq!(fs::read_dir(&root).unwrap().count(), 1);
            server.join().unwrap();
        }
        fs::remove_dir_all(root).unwrap();
    }
}
