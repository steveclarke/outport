(function () {
  "use strict";

  var dashboard = document.getElementById("dashboard");
  var connDot = document.getElementById("connection-dot");
  var connText = document.getElementById("connection-text");

  function setConnection(state) {
    connDot.className = state;
    connText.textContent =
      state === "connected" ? "Connected" : "Reconnecting\u2026";
  }

  function fetchStatus() {
    fetch("/api/status")
      .then(function (res) {
        return res.json();
      })
      .then(function (data) {
        render(data);
      })
      .catch(function () {
        // Will retry via SSE reconnect.
      });
  }

  function connectSSE() {
    var source = new EventSource("/api/events");

    source.addEventListener("registry", function (e) {
      var data = JSON.parse(e.data);
      render(data);
    });

    source.addEventListener("health", function (e) {
      var changes = JSON.parse(e.data);
      updateHealth(changes);
    });

    source.addEventListener("open", function () {
      setConnection("connected");
    });

    source.addEventListener("error", function () {
      setConnection("reconnecting");
    });
  }

  function clearDashboard() {
    while (dashboard.firstChild) {
      dashboard.removeChild(dashboard.firstChild);
    }
  }

  function render(data) {
    clearDashboard();

    var projects = data.projects || {};
    var projectNames = Object.keys(projects).sort();

    for (var p = 0; p < projectNames.length; p++) {
      var projectName = projectNames[p];
      var project = projects[projectName];

      var group = document.createElement("section");
      group.className = "project-group";

      var heading = document.createElement("h2");
      heading.className = "project-name";
      heading.textContent = projectName;
      group.appendChild(heading);

      var instances = project.instances || {};
      var instanceNames = Object.keys(instances).sort(function (a, b) {
        if (a === "main") return -1;
        if (b === "main") return 1;
        return a < b ? -1 : a > b ? 1 : 0;
      });

      for (var i = 0; i < instanceNames.length; i++) {
        var instanceName = instanceNames[i];
        var instance = instances[instanceName];
        group.appendChild(renderInstance(projectName, instanceName, instance));
      }

      dashboard.appendChild(group);
    }
  }

  function renderInstance(projectName, instanceName, instance) {
    var card = document.createElement("div");
    card.className = "instance-card";

    var header = document.createElement("div");
    header.className = "instance-header";

    var badge = document.createElement("span");
    badge.className = "instance-badge";
    badge.textContent = instanceName;
    header.appendChild(badge);

    if (instance.project_dir) {
      var dir = document.createElement("span");
      dir.className = "instance-dir";
      dir.textContent = instance.project_dir;
      header.appendChild(dir);
    }

    card.appendChild(header);

    var services = instance.services || {};
    var serviceNames = Object.keys(services).sort(function (a, b) {
      var aWeb = services[a].url ? 0 : 1;
      var bWeb = services[b].url ? 0 : 1;
      if (aWeb !== bWeb) return aWeb - bWeb;
      return a < b ? -1 : a > b ? 1 : 0;
    });

    for (var s = 0; s < serviceNames.length; s++) {
      var serviceName = serviceNames[s];
      var service = services[serviceName];
      card.appendChild(renderService(projectName, instanceName, serviceName, service));
    }

    return card;
  }

  function renderService(projectName, instanceName, serviceName, service) {
    var row = document.createElement("div");
    row.className = "service-row";

    var dot = document.createElement("span");
    dot.className = "status-dot";
    dot.setAttribute("data-project", projectName);
    dot.setAttribute("data-instance", instanceName);
    dot.setAttribute("data-service", serviceName);
    if (service.up === true) {
      dot.classList.add("up");
    } else if (service.up === false) {
      dot.classList.add("down");
    }
    row.appendChild(dot);

    if (service.url) {
      var link = document.createElement("a");
      link.className = "service-url";
      link.href = service.url;
      link.target = "_blank";
      link.rel = "noopener";
      link.textContent = serviceName;
      row.appendChild(link);
    } else {
      var name = document.createElement("span");
      name.className = "service-name";
      name.textContent = serviceName;
      row.appendChild(name);
    }

    var port = document.createElement("span");
    port.className = "service-port";
    port.textContent = ":" + service.port;
    row.appendChild(port);

    if (service.env_var) {
      var envVar = document.createElement("span");
      envVar.className = "service-envvar";
      envVar.textContent = service.env_var;
      row.appendChild(envVar);
    }

    var spacer = document.createElement("span");
    spacer.className = "service-spacer";
    row.appendChild(spacer);

    return row;
  }

  function updateHealth(changes) {
    for (var i = 0; i < changes.length; i++) {
      var c = changes[i];
      var selector =
        '[data-project="' + c.project + '"]' +
        '[data-instance="' + c.instance + '"]' +
        '[data-service="' + c.service + '"]';
      var dot = document.querySelector(selector);
      if (dot) {
        dot.classList.remove("up", "down");
        dot.classList.add(c.up ? "up" : "down");
      }
    }
  }

  document.addEventListener("DOMContentLoaded", function () {
    fetchStatus();
    connectSSE();
  });
})();
