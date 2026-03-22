(function () {
  "use strict";

  var dashboard = document.getElementById("dashboard");
  var connDot = document.getElementById("conn-dot");
  var connText = document.getElementById("conn-text");
  var headerStats = document.getElementById("header-stats");

  function setConnection(state) {
    if (state === "connected") {
      connDot.className = "header-conn-dot";
      connText.textContent = "Connected";
    } else {
      connDot.className = "header-conn-dot reconnecting";
      connText.textContent = "Reconnecting\u2026";
    }
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

  // ── Helpers ──

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text) node.textContent = text;
    return node;
  }

  function sortInstances(names) {
    return names.slice().sort(function (a, b) {
      if (a === "main") return -1;
      if (b === "main") return 1;
      return a < b ? -1 : a > b ? 1 : 0;
    });
  }

  function classifyServices(services) {
    var web = [];
    var infra = [];
    var names = Object.keys(services).sort();
    for (var i = 0; i < names.length; i++) {
      var name = names[i];
      if (services[name].url) {
        web.push(name);
      } else {
        infra.push(name);
      }
    }
    return { web: web, infra: infra };
  }

  function dotClass(up) {
    if (up === true) return "up";
    if (up === false) return "down";
    return "";
  }

  // ── Health badge ──

  function computeHealth(project) {
    var instances = project.instances || {};
    var total = 0;
    var upCount = 0;
    var hasHealth = false;

    var instNames = Object.keys(instances);
    for (var i = 0; i < instNames.length; i++) {
      var services = instances[instNames[i]].services || {};
      var svcNames = Object.keys(services);
      for (var j = 0; j < svcNames.length; j++) {
        total++;
        var svc = services[svcNames[j]];
        if (svc.up === true) {
          upCount++;
          hasHealth = true;
        } else if (svc.up === false) {
          hasHealth = true;
        }
      }
    }

    var cls = "idle";
    if (hasHealth) {
      cls = upCount === total ? "ok" : "warn";
    }

    return { up: upCount, total: total, cls: cls };
  }

  function computeInstanceHealth(instance) {
    var services = instance.services || {};
    var names = Object.keys(services);
    var hasHealth = false;
    var allUp = true;
    for (var i = 0; i < names.length; i++) {
      var svc = services[names[i]];
      if (svc.up === true) { hasHealth = true; }
      else if (svc.up === false) { hasHealth = true; allUp = false; }
    }
    if (!hasHealth) return "idle";
    return allUp ? "ok" : "warn";
  }

  // ── Render ──

  function render(data) {
    clearDashboard();

    var projects = data.projects || {};
    var projectNames = Object.keys(projects).sort();

    // Update header stats.
    var totalInstances = 0;
    for (var p = 0; p < projectNames.length; p++) {
      var inst = projects[projectNames[p]].instances || {};
      totalInstances += Object.keys(inst).length;
    }
    headerStats.textContent =
      projectNames.length + (projectNames.length === 1 ? " project" : " projects") +
      " \u00b7 " +
      totalInstances + (totalInstances === 1 ? " instance" : " instances");

    for (var pi = 0; pi < projectNames.length; pi++) {
      dashboard.appendChild(renderProject(projectNames[pi], projects[projectNames[pi]]));
    }
  }

  function renderProject(projectName, project) {
    var section = el("section", "project");

    // Sidebar
    var sidebar = el("div", "project-sidebar");
    sidebar.appendChild(el("span", "project-name", projectName));

    var health = computeHealth(project);
    var badge = el("span", "project-health " + health.cls, health.up + "/" + health.total);
    sidebar.appendChild(badge);
    section.appendChild(sidebar);

    // Instances
    var instancesDiv = el("div", "project-instances");
    var instances = project.instances || {};
    var instNames = sortInstances(Object.keys(instances));

    // Separate main from worktrees
    var mainName = null;
    var worktreeNames = [];
    for (var i = 0; i < instNames.length; i++) {
      if (instNames[i] === "main") {
        mainName = instNames[i];
      } else {
        worktreeNames.push(instNames[i]);
      }
    }

    // Render main instance (always visible)
    if (mainName) {
      instancesDiv.appendChild(renderInstance(projectName, mainName, instances[mainName]));
    }

    // Render worktree instances in a collapsible toggle
    if (worktreeNames.length > 0) {
      var wtToggle = document.createElement("details");
      wtToggle.className = "wt-toggle";

      var wtSummary = document.createElement("summary");
      wtSummary.textContent = worktreeNames.length + (worktreeNames.length === 1 ? " worktree " : " worktrees ");

      // Add worktree dots — reflect actual health of each worktree
      var wtDots = el("span", "wt-dots");
      for (var wi = 0; wi < worktreeNames.length; wi++) {
        var wtInst = instances[worktreeNames[wi]];
        var wtHealth = computeInstanceHealth(wtInst);
        var wtDot = el("span", "wt-dot");
        if (wtHealth === "warn") wtDot.classList.add("down");
        else if (wtHealth === "idle") wtDot.classList.add("idle");
        wtDots.appendChild(wtDot);
      }
      wtSummary.appendChild(wtDots);
      wtToggle.appendChild(wtSummary);

      var wtInstances = el("div", "wt-instances");
      for (var wj = 0; wj < worktreeNames.length; wj++) {
        wtInstances.appendChild(renderInstance(projectName, worktreeNames[wj], instances[worktreeNames[wj]]));
      }
      wtToggle.appendChild(wtInstances);
      instancesDiv.appendChild(wtToggle);
    }

    section.appendChild(instancesDiv);
    return section;
  }

  function renderInstance(projectName, instanceName, instance) {
    var card = el("div", "instance");

    // Instance header
    var head = el("div", "inst-head" + (instanceName === "main" ? " main" : ""), instanceName);
    card.appendChild(head);

    var services = instance.services || {};
    var classified = classifyServices(services);

    // Web services (those with url) — shown directly
    for (var w = 0; w < classified.web.length; w++) {
      var wName = classified.web[w];
      var wSvc = services[wName];
      card.appendChild(renderWebService(projectName, instanceName, wName, wSvc));
    }

    // Infra services — collapsed in details toggle
    if (classified.infra.length > 0) {
      var details = document.createElement("details");
      details.className = "infra-toggle";

      var summary = document.createElement("summary");

      // Inline dots
      var idots = el("span", "idots");
      for (var d = 0; d < classified.infra.length; d++) {
        var iName = classified.infra[d];
        var iSvc = services[iName];
        var idot = el("span", "idot");
        var dc = dotClass(iSvc.up);
        if (dc) idot.classList.add(dc);
        idot.setAttribute("data-project", projectName);
        idot.setAttribute("data-instance", instanceName);
        idot.setAttribute("data-service", iName);
        idots.appendChild(idot);
      }
      summary.appendChild(idots);

      var countText = document.createTextNode(
        classified.infra.length + " more " + (classified.infra.length === 1 ? "service" : "services")
      );
      summary.appendChild(countText);
      details.appendChild(summary);

      // Infra rows
      for (var ir = 0; ir < classified.infra.length; ir++) {
        var irName = classified.infra[ir];
        var irSvc = services[irName];
        details.appendChild(renderInfraRow(projectName, instanceName, irName, irSvc));
      }

      card.appendChild(details);
    }

    return card;
  }

  function renderWebService(projectName, instanceName, serviceName, service) {
    var row = el("div", "svc");

    var dot = el("span", "dot");
    var dc = dotClass(service.up);
    if (dc) dot.classList.add(dc);
    dot.setAttribute("data-project", projectName);
    dot.setAttribute("data-instance", instanceName);
    dot.setAttribute("data-service", serviceName);
    row.appendChild(dot);

    var link = el("a", "svc-link", service.hostname || serviceName);
    link.href = service.url;
    link.target = "_blank";
    link.rel = "noopener";
    row.appendChild(link);

    row.appendChild(el("span", "svc-port", String(service.port)));
    row.appendChild(el("span", "svc-envvar", service.env_var || ""));

    return row;
  }

  function renderInfraRow(projectName, instanceName, serviceName, service) {
    var row = el("div", "infra-row");

    var dot = el("span", "dot");
    var dc = dotClass(service.up);
    if (dc) dot.classList.add(dc);
    dot.setAttribute("data-project", projectName);
    dot.setAttribute("data-instance", instanceName);
    dot.setAttribute("data-service", serviceName);
    row.appendChild(dot);

    row.appendChild(el("span", null, serviceName));
    row.appendChild(el("span", "svc-port", String(service.port)));
    row.appendChild(el("span", "svc-envvar", service.env_var || ""));

    return row;
  }

  // ── Health updates (targeted dot swap) ──

  function updateHealth(changes) {
    for (var i = 0; i < changes.length; i++) {
      var c = changes[i];
      var selector =
        '[data-project="' + c.project + '"]' +
        '[data-instance="' + c.instance + '"]' +
        '[data-service="' + c.service + '"]';
      var dots = document.querySelectorAll(selector);
      for (var j = 0; j < dots.length; j++) {
        dots[j].classList.remove("up", "down");
        dots[j].classList.add(c.up ? "up" : "down");
      }
    }
  }

  document.addEventListener("DOMContentLoaded", function () {
    fetchStatus();
    connectSSE();
  });
})();
