package com.clawbench.app;

import org.junit.Test;

import cn.jpush.android.service.JCommonService;

import static org.junit.Assert.*;

/**
 * Unit tests for UserService.
 *
 * UserService is a minimal class that extends JCommonService to satisfy
 * JPush SDK's discovery mechanism. It has no custom logic, but we verify:
 * - It extends JCommonService (required for JPush SDK compatibility)
 * - It can be instantiated (no abstract methods or missing implementations)
 * - The class exists in the correct package
 */
public class UserServiceTest {

    // =====================================================
    // Test 1: UserService extends JCommonService
    // =====================================================

    @Test
    public void userService_extendsJCommonService() {
        assertTrue("UserService should extend JCommonService",
                JCommonService.class.isAssignableFrom(UserService.class));
    }

    // =====================================================
    // Test 2: UserService is in correct package
    // =====================================================

    @Test
    public void userService_correctPackage() {
        assertEquals("com.clawbench.app", UserService.class.getPackage().getName());
    }

    // =====================================================
    // Test 3: UserService is a concrete class (not abstract)
    // =====================================================

    @Test
    public void userService_isConcreteClass() {
        assertFalse("UserService should not be abstract",
                java.lang.reflect.Modifier.isAbstract(UserService.class.getModifiers()));
    }

    // =====================================================
    // Test 4: UserService is public
    // =====================================================

    @Test
    public void userService_isPublic() {
        assertTrue("UserService should be public",
                java.lang.reflect.Modifier.isPublic(UserService.class.getModifiers()));
    }

    // =====================================================
    // Test 5: UserService extends Service hierarchy
    // =====================================================

    @Test
    public void userService_extendsServiceHierarchy() {
        // JCommonService → Service, so UserService should be assignable to Service
        assertTrue("UserService should be assignable to android.app.Service",
                android.app.Service.class.isAssignableFrom(UserService.class));
    }
}
