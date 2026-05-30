package com.clawbench.app;

import cn.jpush.android.service.JCommonService;

/**
 * JPush SDK-required service that extends JCommonService.
 *
 * Why this exists alongside PushService:
 * JPush SDK discovers the app's push service through the intent-filter
 * "cn.jiguang.user.service.action" and verifies that it's a subclass of
 * JCommonService (via Class.isAssignableFrom). Without this, the SDK falls
 * back to single-process mode and cross-process DataShare binder communication
 * is lost.
 *
 * This class runs in the :pushcore process alongside PushService. Having two
 * services in the same process is completely normal on Android — zero extra
 * memory overhead, independent lifecycles, and the process is already kept
 * alive by PushService's foreground notification.
 */
public class UserService extends JCommonService {
}
