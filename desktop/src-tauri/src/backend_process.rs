use crate::{backend, token};
use std::{
    io::{BufRead, BufReader, Read, Write},
    net::{SocketAddr, TcpStream},
    process::{Child, Command, Stdio},
    sync::{mpsc, Arc, Mutex},
    thread,
    time::Duration,
};

pub struct BackendProcess(Mutex<Option<Child>>);

impl BackendProcess {
    pub fn start() -> Result<Self, String> {
        let (base, token_path) = backend()?;
        let address: SocketAddr = base
            .strip_prefix("http://")
            .ok_or("Invalid backend address")?
            .parse()
            .map_err(|_| "Invalid backend address")?;
        if TcpStream::connect_timeout(&address, Duration::from_millis(200)).is_ok() {
            return Err(format!("Port {} is already in use. Close the other DevOrbit window or change DEVORBIT_PORT.", address.port()));
        }

        let executable = std::env::current_exe()
            .map_err(|error| format!("Cannot locate DevOrbit: {error}"))?
            .with_file_name(if cfg!(windows) {
                "devorbit.exe"
            } else {
                "devorbit"
            });
        let mut child = Command::new(&executable)
            .env("DEVORBIT_DESKTOP_CHILD", "1")
            .env("DEVORBIT_DATA_DIR", super::data_dir()?)
            .env("DEVORBIT_AUTH_TOKEN_PATH", &token_path)
            .current_dir(executable.parent().ok_or("Invalid backend path")?)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .map_err(|error| format!("Cannot start bundled backend: {error}"))?;

        let (ready_tx, ready_rx) = mpsc::channel();
        let stdout = child.stdout.take().ok_or("Cannot read backend readiness")?;
        thread::spawn(move || {
            let mut line = String::new();
            let _ = ready_tx.send(
                BufReader::new(stdout).read_line(&mut line).is_ok()
                    && line.trim_end() == "DEVORBIT_READY",
            );
        });

        let errors = Arc::new(Mutex::new(Vec::new()));
        let recent = Arc::clone(&errors);
        let mut stderr = child.stderr.take().ok_or("Cannot read backend errors")?;
        thread::spawn(move || {
            let mut buffer = [0; 1024];
            while let Ok(n @ 1..) = stderr.read(&mut buffer) {
                let mut recent = recent.lock().unwrap();
                recent.extend_from_slice(&buffer[..n]);
                if recent.len() > 4096 {
                    let excess = recent.len() - 4096;
                    recent.drain(..excess);
                }
            }
        });

        let mut own_backend_ready = false;
        for _ in 0..150 {
            if let Ok(Some(status)) = child.try_wait() {
                thread::sleep(Duration::from_millis(20));
                return Err(format!(
                    "Backend exited ({status}). {}",
                    String::from_utf8_lossy(&errors.lock().unwrap())
                ));
            }
            if matches!(ready_rx.try_recv(), Ok(true)) {
                own_backend_ready = true;
            }
            if own_backend_ready && healthy(address) && token(&token_path).is_ok() {
                return Ok(Self(Mutex::new(Some(child))));
            }
            thread::sleep(Duration::from_millis(100));
        }
        stop_child(&mut child);
        Err(format!(
            "Backend did not become ready at {base}. {}",
            String::from_utf8_lossy(&errors.lock().unwrap())
        ))
    }

    pub fn stop(&self) {
        if let Some(mut child) = self.0.lock().unwrap().take() {
            stop_child(&mut child);
        }
    }
}

fn healthy(address: SocketAddr) -> bool {
    let Ok(mut stream) = TcpStream::connect_timeout(&address, Duration::from_millis(200)) else {
        return false;
    };
    let _ = stream.set_read_timeout(Some(Duration::from_millis(200)));
    if write!(
        stream,
        "GET /health HTTP/1.1\r\nHost: {address}\r\nConnection: close\r\n\r\n"
    )
    .is_err()
    {
        return false;
    }
    let mut status = String::new();
    BufReader::new(stream).read_line(&mut status).is_ok() && status.starts_with("HTTP/1.1 200 ")
}

fn stop_child(child: &mut Child) {
    drop(child.stdin.take());
    for _ in 0..150 {
        if matches!(child.try_wait(), Ok(Some(_))) {
            return;
        }
        thread::sleep(Duration::from_millis(100));
    }
    let _ = child.kill();
    let _ = child.wait();
}

#[cfg(test)]
mod tests {
    use super::healthy;
    use std::{
        io::{BufRead, BufReader, Write},
        net::TcpListener,
        thread,
    };

    #[test]
    fn readiness_requires_a_successful_health_response() {
        for (response, expected) in [
            ("HTTP/1.1 200 OK\r\n\r\n", true),
            ("HTTP/1.1 503 Unavailable\r\n\r\n", false),
        ] {
            let listener = TcpListener::bind("127.0.0.1:0").unwrap();
            let address = listener.local_addr().unwrap();
            let server = thread::spawn(move || {
                let (mut stream, _) = listener.accept().unwrap();
                let mut reader = BufReader::new(stream.try_clone().unwrap());
                let mut line = String::new();
                while reader.read_line(&mut line).unwrap() > 0 && line != "\r\n" {
                    line.clear();
                }
                stream.write_all(response.as_bytes()).unwrap();
            });
            assert_eq!(healthy(address), expected);
            server.join().unwrap();
        }
    }
}
