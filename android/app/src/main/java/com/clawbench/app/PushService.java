package com.clawbench.app;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.pm.ServiceInfo;
import android.os.Build;
import android.os.IBinder;
import android.text.TextUtils;

import androidx.annotation.Nullable;
import androidx.core.app.NotificationCompat;

import cn.jiguang.android.IDataShare;
import cn.jiguang.api.JCoreManager;
import cn.jiguang.internal.JConstants;
import cn.jiguang.internal.JCoreInternalHelper;
import cn.jpush.android.service.DataShare;

/**
 * Replacement for the default JCommonService that JPush SDK requires.
 * Runs in the :pushcore process.
 *
 * Why not extend JCommonService?
 * JCommonService declares ALL lifecycle methods as 'final', making it impossible
 * to override onCreate() or onStartCommand() to add foreground service promotion.
 * Without foreground service, the :pushcore process is killed within minutes on
 * aggressive OEM ROMs (Xiaomi HyperOS, Huawei EMUI, OPPO ColorOS).
 *
 * This class replicates JCommonService's essential initialization:
 * 1. onCreate(): JCoreManager.setStartId() + JConstants.getAppContext() + DataShare binder
 * 2. onStartCommand(): Dispatches intents to JCoreInternalHelper.onEvent() + returns START_STICKY
 * 3. onBind(): Returns the DataShare binder for cross-process communication
 *
 * Additionally promotes itself to foreground service with a persistent notification,
 * making the :pushcore process much harder for the system to kill.
 */
public class PushService extends Service {

    private static final String TAG = "ClawBench";
    private static final String CHANNEL_ID = "clawbench_push";
    private static final int NOTIFICATION_ID = 20001;

    /** SharedPreferences key for persistent notification toggle. Default: true */
    static final String PREF_KEY_PERSISTENT_NOTIFICATION = "push_persistent_notification";

    /** Intent actions for toggling foreground service from the main process */
    private static final String ACTION_ENABLE_FGS = "com.clawbench.app.PushService.ENABLE_FGS";
    private static final String ACTION_DISABLE_FGS = "com.clawbench.app.PushService.DISABLE_FGS";

    /**
     * Action sent when the user swipes away the persistent notification.
     * The service re-posts the notification to maintain foreground state.
     * On some OEM ROMs (HyperOS, EMUI), even setOngoing(true) notifications
     * can be swiped away — this re-promotes to foreground automatically.
     */
    private static final String ACTION_NOTIFICATION_DISMISSED = "com.clawbench.app.PushService.NOTIFICATION_DISMISSED";

    private IDataShare.Stub mBinder;
    private boolean isForeground = false;

    /**
     * Set to true when we explicitly want foreground mode (user toggle ON).
     * Distinct from isForeground which tracks the actual system state —
     * the user can swipe away the notification (dropping us out of foreground)
     * while the preference is still enabled.
     */
    private boolean wantsForeground = false;

    @Override
    public void onCreate() {
        super.onCreate();
        // Replicate JCommonService.onCreate() initialization
        JCoreManager.setStartId(getApplicationContext());
        JConstants.getAppContext(getApplicationContext());
        // Create a fresh DataShare binder per Service instance.
        // DataShare extends IDataShare.Stub and routes calls through JCoreManager
        // (which is static/singleton), so a new instance is safe.
        mBinder = new DataShare();

        // Check user preference before promoting to foreground
        if (isPersistentNotificationEnabled()) {
            wantsForeground = true;
            startForegroundCompat();
            AppLog.i(TAG, "PushService: onCreate, promoted to foreground service");
        } else {
            wantsForeground = false;
            AppLog.i(TAG, "PushService: onCreate, persistent notification disabled, running as background service");
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        // Handle FGS toggle actions from the main process
        if (intent != null) {
            String action = intent.getAction();
            if (ACTION_ENABLE_FGS.equals(action)) {
                wantsForeground = true;
                startForegroundCompat();
                AppLog.i(TAG, "PushService: FGS enabled via settings");
                return START_STICKY;
            } else if (ACTION_DISABLE_FGS.equals(action)) {
                wantsForeground = false;
                stopForegroundCompat();
                AppLog.i(TAG, "PushService: FGS disabled via settings");
                return START_STICKY;
            } else if (ACTION_NOTIFICATION_DISMISSED.equals(action)) {
                // User swiped away the persistent notification — re-post it
                // to maintain foreground service state. Without this, the
                // :pushcore process loses FGS protection and gets killed.
                if (wantsForeground) {
                    AppLog.i(TAG, "PushService: notification dismissed, re-posting foreground notification");
                    // Reset isForeground so startForegroundCompat() will call startForeground()
                    isForeground = false;
                    startForegroundCompat();
                }
                return START_STICKY;
            }
        }

        // Replicate JCommonService.onStartCommand() intent dispatching logic.
        // Original: dispatches intent action + extras to JCoreInternalHelper,
        // then returns START_REDELIVER_INTENT.
        // Note: we only dispatch JPush SDK actions (not our internal FGS actions).
        if (intent != null && !TextUtils.isEmpty(intent.getAction())) {
            String action = intent.getAction();
            if (!ACTION_ENABLE_FGS.equals(action) && !ACTION_DISABLE_FGS.equals(action)
                    && !ACTION_NOTIFICATION_DISMISSED.equals(action)) {
                JCoreInternalHelper.getInstance().onEvent(
                        getApplicationContext(),
                        "JCore",
                        2,                      // cmd = 2 (matches JCommonService)
                        true,                   // isGlobal = true
                        action,
                        intent.getExtras(),
                        new Object[0]
                );
            }
        } else {
            AppLog.i(TAG, "PushService: onStartCommand intent is empty or action is empty");
        }

        // Re-ensure foreground state if wanted but lost (e.g. after system kill & restart,
        // or user swiped away the notification on some ROMs).
        if (wantsForeground && !isForeground) {
            startForegroundCompat();
        }

        // START_STICKY: Android recreates the service with null intent after kill.
        // This is critical for :pushcore survival on OEM ROMs where
        // START_REDELIVER_INTENT fails because there's no pending intent to redeliver.
        return START_STICKY;
    }

    @Nullable
    @Override
    public IBinder onBind(Intent intent) {
        return mBinder;
    }

    @Override
    public void onDestroy() {
        AppLog.i(TAG, "PushService: onDestroy");
        if (isForeground) {
            stopForeground(true);
            isForeground = false;
        }
        super.onDestroy();
    }

    /**
     * Check whether the persistent notification preference is enabled.
     * Reads from SharedPreferences, defaults to true.
     */
    private boolean isPersistentNotificationEnabled() {
        SharedPreferences prefs = getSharedPreferences("clawbench_prefs", MODE_PRIVATE);
        return prefs.getBoolean(PREF_KEY_PERSISTENT_NOTIFICATION, true);
    }

    /**
     * Start this service as foreground with a minimal persistent notification.
     * Uses foregroundServiceType="remoteMessaging" (declared in manifest) which
     * has no time limit — the service can run indefinitely.
     *
     * Always calls startForeground() even if already in foreground state.
     * This handles the case where the user swiped away the notification —
     * calling startForeground() again re-posts it, and is a no-op if already
     * in foreground with the same notification ID.
     */
    private void startForegroundCompat() {
        createNotificationChannel();

        // Tap the notification to launch MainActivity (re-opens the app if the
        // main process was killed). This is important because when the main
        // process is dead but :pushcore is alive, the user sees this orphaned
        // notification — tapping it should bring the app back to life.
        Intent launchIntent = new Intent(this, MainActivity.class);
        launchIntent.setAction(Intent.ACTION_MAIN);
        launchIntent.addCategory(Intent.CATEGORY_LAUNCHER);
        launchIntent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_RESET_TASK_IF_NEEDED);
        PendingIntent contentIntent = PendingIntent.getActivity(this, 0, launchIntent,
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);

        Notification notification = new NotificationCompat.Builder(this, CHANNEL_ID)
                .setContentTitle("ClawBench")
                .setContentText("推送服务运行中")
                .setSmallIcon(R.drawable.ic_notification)
                .setOngoing(true)
                .setSilent(true)
                .setContentIntent(contentIntent)
                .setDeleteIntent(createNotificationDeletedIntent())
                .build();

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            startForeground(NOTIFICATION_ID, notification,
                    ServiceInfo.FOREGROUND_SERVICE_TYPE_REMOTE_MESSAGING);
        } else if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForeground(NOTIFICATION_ID, notification);
        }
        // Pre-O: no foreground service concept, service runs normally
        isForeground = true;
    }

    /**
     * Stop the foreground service, removing the persistent notification.
     * The service continues running as a background service.
     */
    private void stopForegroundCompat() {
        if (!isForeground) return; // Already in background
        stopForeground(true);
        isForeground = false;
    }

    /**
     * Create the notification channel for the push service notification.
     * Uses IMPORTANCE_MIN to show the notification without sound/alert.
     */
    private void createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationManager nm = (NotificationManager) getSystemService(Context.NOTIFICATION_SERVICE);
            if (nm != null && nm.getNotificationChannel(CHANNEL_ID) == null) {
                NotificationChannel channel = new NotificationChannel(
                        CHANNEL_ID,
                        "推送服务",
                        NotificationManager.IMPORTANCE_MIN
                );
                channel.setDescription("极光推送后台服务");
                channel.setShowBadge(false);
                nm.createNotificationChannel(channel);
            }
        }
    }

    /**
     * Create a PendingIntent that fires when the user swipes away the notification.
     * This sends ACTION_NOTIFICATION_DISMISSED back to PushService, which re-posts
     * the foreground notification to maintain FGS protection.
     *
     * On most Android versions, setOngoing(true) prevents swipe dismissal, but
     * some OEM ROMs (Xiaomi HyperOS, Huawei EMUI) allow it anyway. Without this
     * deleteIntent, a swiped-away notification means the :pushcore process silently
     * drops out of foreground state and becomes vulnerable to being killed.
     */
    private PendingIntent createNotificationDeletedIntent() {
        Intent dismissedIntent = new Intent(this, PushService.class);
        dismissedIntent.setAction(ACTION_NOTIFICATION_DISMISSED);
        return PendingIntent.getService(this, 0, dismissedIntent,
                PendingIntent.FLAG_UPDATE_CURRENT | PendingIntent.FLAG_IMMUTABLE);
    }

    /**
     * Toggle the persistent notification from the main process.
     * Writes the preference to SharedPreferences and sends an Intent
     * to PushService to enable/disable foreground service mode.
     *
     * @param context  Application context
     * @param enabled  true to show persistent notification (FGS), false to hide
     */
    public static void setPersistentNotification(Context context, boolean enabled) {
        // Write preference
        SharedPreferences prefs = context.getSharedPreferences("clawbench_prefs", Context.MODE_PRIVATE);
        prefs.edit().putBoolean(PREF_KEY_PERSISTENT_NOTIFICATION, enabled).apply();

        // Send intent to PushService in :pushcore process.
        // When enabling FGS, use startForegroundService() on API 26+ to satisfy
        // Android 12+ background service start restrictions. PushService handles
        // the startForeground() call in its onStartCommand() within 5 seconds.
        // When disabling, startService() is fine since the service is already running.
        Intent intent = new Intent(context, PushService.class);
        intent.setAction(enabled ? ACTION_ENABLE_FGS : ACTION_DISABLE_FGS);
        try {
            if (enabled && Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent);
            } else {
                context.startService(intent);
            }
        } catch (Exception e) {
            AppLog.w(TAG, "Failed to toggle PushService FGS: " + e.getMessage());
        }
    }

    /**
     * Read the persistent notification preference.
     * Can be called from any process.
     *
     * @param context  Application context
     * @return true if persistent notification is enabled (default)
     */
    public static boolean isPersistentNotificationEnabled(Context context) {
        SharedPreferences prefs = context.getSharedPreferences("clawbench_prefs", Context.MODE_PRIVATE);
        return prefs.getBoolean(PREF_KEY_PERSISTENT_NOTIFICATION, true);
    }
}
