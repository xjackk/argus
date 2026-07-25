import React from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { getTheme } from "./data/theme";
import "./styles.css";

// Apply the saved theme before first paint to avoid a flash.
document.documentElement.dataset.theme = getTheme();

const container = document.getElementById("root");
const root = createRoot(container!);

root.render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
