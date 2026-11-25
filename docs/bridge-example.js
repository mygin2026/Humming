/**
 * Humming Bridge 最小 Promise 封装示例（H5 侧）。
 * 依赖 Native：window.webkit.messageHandlers.humming
 */
(function (global) {
  const DEFAULT_TIMEOUT_MS = 8000;

  function hummingCall(method, params, timeoutMs) {
    params = params || {};
    timeoutMs = timeoutMs || DEFAULT_TIMEOUT_MS;

    return new Promise(function (resolve, reject) {
      if (
        !global.webkit ||
        !global.webkit.messageHandlers ||
        !global.webkit.messageHandlers.humming
      ) {
        reject({ code: "unavailable", message: "Bridge 不可用" });
        return;
      }

      var id =
        "req-" +
        Date.now().toString(36) +
        "-" +
        Math.random().toString(36).slice(2, 8);

      var timer = setTimeout(function () {
        cleanup();
        reject({ code: "timeout", message: "Bridge 调用超时" });
      }, timeoutMs);

      function onEvent(event) {
        var detail = event && event.detail;
        if (!detail || detail.id !== id) {
          return;
        }
        cleanup();
        if (detail.ok) {
          resolve(detail.result);
        } else {
          reject(detail.error || { code: "error", message: "未知错误" });
        }
      }

      function cleanup() {
        clearTimeout(timer);
        global.removeEventListener("humming://bridge", onEvent);
      }

      global.addEventListener("humming://bridge", onEvent);
      try {
        global.webkit.messageHandlers.humming.postMessage({
          id: id,
          method: method,
          params: params,
        });
      } catch (err) {
        cleanup();
        reject({ code: "post_failed", message: String(err) });
      }
    });
  }

  global.HummingBridge = {
    getAppInfo: function () {
      return hummingCall("getAppInfo", {});
    },
    getNetworkStatus: function () {
      return hummingCall("getNetworkStatus", {});
    },
    getCapabilities: function () {
      return hummingCall("getCapabilities", {});
    },
    openExternalURL: function (url) {
      return hummingCall("openExternalURL", { url: url });
    },
  };
})(typeof window !== "undefined" ? window : globalThis);
