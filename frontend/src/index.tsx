import "./app.css";
import { render } from "solid-js/web";
import { HashRouter, Route } from "@solidjs/router";
import { App } from "./App";
import Connections from "./pages/Connections";
import Storage from "./pages/Storage";
import Settings from "./pages/Settings";

render(
  () => (
    <HashRouter root={App}>
      <Route path="/" component={Connections} />
      <Route path="/storage" component={Storage} />
      <Route path="/settings" component={Settings} />
    </HashRouter>
  ),
  document.getElementById("root")!
);
