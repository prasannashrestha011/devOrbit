import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import QuickControls from "./QuickControls";
import "./styles.css";

const quickControls = window.location.hash === "#quick-controls";
document.body.classList.toggle("quick-controls-window", quickControls);

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    {quickControls ? <QuickControls /> : <App />}
  </React.StrictMode>,
);
