(function () {
  "use strict";

  var dashboard = document.getElementById("dashboard");
  var connDot = document.getElementById("conn-dot");
  var connText = document.getElementById("conn-text");
  var headerStats = document.getElementById("header-stats");
  var toggleBtn = document.getElementById("toggle-inactive");

  var showInactive = false;
  var lastData = null;

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

  // Health badge only counts services that have actually been health-checked.
  // Services with up === null/undefined (not running, never checked) are excluded
  // from the count — they're not a problem, just dormant.
  function computeHealth(project) {
    var instances = project.instances || {};
    var checked = 0;
    var upCount = 0;

    var instNames = Object.keys(instances);
    for (var i = 0; i < instNames.length; i++) {
      var services = instances[instNames[i]].services || {};
      var svcNames = Object.keys(services);
      for (var j = 0; j < svcNames.length; j++) {
        var svc = services[svcNames[j]];
        if (svc.up === true) { checked++; upCount++; }
        else if (svc.up === false) { checked++; }
      }
    }

    var cls = "idle";
    if (checked > 0) {
      cls = upCount === checked ? "ok" : "warn";
    }

    return { up: upCount, total: checked, cls: cls };
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

  // A project is "inactive" only if ALL services have been health-checked
  // and ALL are down (up === false). If any service is up, or if no health
  // data exists yet (up is null/undefined), it's considered active.
  function isProjectActive(project) {
    var instances = project.instances || {};
    var instNames = Object.keys(instances);
    var hasAnyHealth = false;
    var hasAnyUp = false;
    for (var i = 0; i < instNames.length; i++) {
      var services = instances[instNames[i]].services || {};
      var svcNames = Object.keys(services);
      for (var j = 0; j < svcNames.length; j++) {
        var svc = services[svcNames[j]];
        if (svc.up === true) { hasAnyUp = true; return true; }
        if (svc.up === false) { hasAnyHealth = true; }
      }
    }
    // No health data at all — treat as active (just registered, not checked yet)
    if (!hasAnyHealth) return true;
    // All checked, none up
    return false;
  }

  // ── Render ──

  function render(data) {
    lastData = data;
    clearDashboard();

    var projects = data.projects || {};
    var projectNames = Object.keys(projects).sort();

    // Split into active/inactive
    var activeNames = [];
    var inactiveCount = 0;
    for (var p = 0; p < projectNames.length; p++) {
      if (isProjectActive(projects[projectNames[p]])) {
        activeNames.push(projectNames[p]);
      } else {
        inactiveCount++;
        if (showInactive) activeNames.push(projectNames[p]);
      }
    }

    // Update header stats
    var totalInstances = 0;
    for (var s = 0; s < projectNames.length; s++) {
      var inst = projects[projectNames[s]].instances || {};
      totalInstances += Object.keys(inst).length;
    }
    headerStats.textContent =
      projectNames.length + (projectNames.length === 1 ? " project" : " projects") +
      " \u00b7 " +
      totalInstances + (totalInstances === 1 ? " instance" : " instances");

    // Update toggle button
    if (inactiveCount > 0) {
      toggleBtn.style.display = "";
      toggleBtn.textContent = showInactive
        ? "Hide inactive (" + inactiveCount + ")"
        : "Show inactive (" + inactiveCount + ")";
      toggleBtn.className = "header-toggle" + (showInactive ? " active" : "");
    } else {
      toggleBtn.style.display = "none";
    }

    for (var pi = 0; pi < activeNames.length; pi++) {
      dashboard.appendChild(renderProject(activeNames[pi], projects[activeNames[pi]]));
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

  // ── Health updates ──

  function updateHealth(changes) {
    if (!lastData) return;

    // Update the cached data with new health values
    var needsRerender = false;
    for (var i = 0; i < changes.length; i++) {
      var c = changes[i];
      var proj = lastData.projects && lastData.projects[c.project];
      if (!proj) continue;
      var inst = proj.instances && proj.instances[c.instance];
      if (!inst) continue;
      var svc = inst.services && inst.services[c.service];
      if (svc) {
        var wasUp = svc.up;
        svc.up = c.up;
        // If a service went from down/unknown to up or vice versa,
        // the active/inactive classification may have changed
        if (wasUp !== c.up) needsRerender = true;
      }
    }

    // Re-render to pick up active/inactive changes and update all dots
    if (needsRerender) {
      render(lastData);
    }
  }

  document.addEventListener("DOMContentLoaded", function () {
    fetchStatus();
    connectSSE();

    toggleBtn.addEventListener("click", function () {
      showInactive = !showInactive;
      if (lastData) render(lastData);
    });
  });
})();
