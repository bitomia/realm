get_filename_component(REALM_CMAKE_DIR "${CMAKE_CURRENT_LIST_FILE}" PATH)
get_filename_component(REALM_PREFIX "${REALM_CMAKE_DIR}/../.." ABSOLUTE)

find_package(Threads REQUIRED)

# Determine shared library extension and names based on platform
if(WIN32)
    set(REALM_DAEMON_LIB_NAME "realm-daemon.dll")
    set(REALM_DAEMON_IMPLIB_NAME "realm-daemon.lib")
    set(REALM_DAEMON_HEADER_NAME "realm-daemon.h")

    set(REALM_CLIENT_LIB_NAME "realm-client.dll")
    set(REALM_CLIENT_IMPLIB_NAME "realm-client.lib")
    set(REALM_CLIENT_HEADER_NAME "realm-client.h")
else()
    set(REALM_DAEMON_LIB_NAME "librealm-daemon.so")
    set(REALM_DAEMON_HEADER_NAME "realm-daemon.h")

    set(REALM_CLIENT_LIB_NAME "librealm-client.so")
    set(REALM_CLIENT_HEADER_NAME "realm-client.h")
endif()

# Define the Realm::Daemon imported library target
if(NOT TARGET Realm::Daemon)
    if(WIN32)
        add_library(Realm::Daemon SHARED IMPORTED)
        set_target_properties(Realm::Daemon PROPERTIES
            IMPORTED_LOCATION "${REALM_PREFIX}/bin/${REALM_DAEMON_LIB_NAME}"
            IMPORTED_IMPLIB "${REALM_PREFIX}/bin/${REALM_DAEMON_IMPLIB_NAME}"
            INTERFACE_INCLUDE_DIRECTORIES "${REALM_PREFIX}/bin"
            INTERFACE_LINK_LIBRARIES "Threads::Threads;${CMAKE_DL_LIBS}"
        )
    else()
        add_library(Realm::Daemon SHARED IMPORTED)
        set_target_properties(Realm::Daemon PROPERTIES
            IMPORTED_LOCATION "${REALM_PREFIX}/bin/${REALM_DAEMON_LIB_NAME}"
            INTERFACE_INCLUDE_DIRECTORIES "${REALM_PREFIX}/bin"
            INTERFACE_LINK_LIBRARIES "Threads::Threads;${CMAKE_DL_LIBS}"
        )
    endif()

    # Add platform-specific libraries that Go C archives typically need
    if(UNIX AND NOT APPLE)
        # Linux
        set_property(TARGET Realm::Daemon APPEND PROPERTY
            INTERFACE_LINK_LIBRARIES "m"
        )
    elseif(APPLE)
        # macOS - may need additional frameworks
        set_property(TARGET Realm::Daemon APPEND PROPERTY
            INTERFACE_LINK_LIBRARIES "-framework CoreFoundation"
        )
    endif()
endif()

# Define the Realm::Client imported library target
if(NOT TARGET Realm::Client)
    if(WIN32)
        add_library(Realm::Client SHARED IMPORTED)
        set_target_properties(Realm::Client PROPERTIES
            IMPORTED_LOCATION "${REALM_PREFIX}/bin/${REALM_CLIENT_LIB_NAME}"
            IMPORTED_IMPLIB "${REALM_PREFIX}/bin/${REALM_CLIENT_IMPLIB_NAME}"
            INTERFACE_INCLUDE_DIRECTORIES "${REALM_PREFIX}/bin"
            INTERFACE_LINK_LIBRARIES "Threads::Threads;${CMAKE_DL_LIBS}"
        )
    else()
        add_library(Realm::Client SHARED IMPORTED)
        set_target_properties(Realm::Client PROPERTIES
            IMPORTED_LOCATION "${REALM_PREFIX}/bin/${REALM_CLIENT_LIB_NAME}"
            INTERFACE_INCLUDE_DIRECTORIES "${REALM_PREFIX}/bin"
            INTERFACE_LINK_LIBRARIES "Threads::Threads;${CMAKE_DL_LIBS}"
        )
    endif()

    # Add platform-specific libraries that Go C archives typically need
    if(UNIX AND NOT APPLE)
        # Linux
        set_property(TARGET Realm::Client APPEND PROPERTY
            INTERFACE_LINK_LIBRARIES "m"
        )
    elseif(APPLE)
        # macOS - may need additional frameworks
        set_property(TARGET Realm::Client APPEND PROPERTY
            INTERFACE_LINK_LIBRARIES "-framework CoreFoundation"
        )
    endif()
endif()

# Set package variables
set(Realm_FOUND TRUE)
set(Realm_LIBRARIES Realm::Daemon Realm::Client)
set(Realm_INCLUDE_DIRS "${REALM_PREFIX}/bin")

# Verify that the library files exist
if(NOT EXISTS "${REALM_PREFIX}/bin/${REALM_DAEMON_LIB_NAME}")
    message(FATAL_ERROR "Realm daemon library not found at ${REALM_PREFIX}/bin/${REALM_DAEMON_LIB_NAME}")
endif()
if(NOT EXISTS "${REALM_PREFIX}/bin/${REALM_DAEMON_HEADER_NAME}")
    message(FATAL_ERROR "Realm daemon header not found at ${REALM_PREFIX}/bin/${REALM_DAEMON_HEADER_NAME}")
endif()
if(NOT EXISTS "${REALM_PREFIX}/bin/${REALM_CLIENT_LIB_NAME}")
    message(FATAL_ERROR "Realm client library not found at ${REALM_PREFIX}/bin/${REALM_CLIENT_LIB_NAME}")
endif()
if(NOT EXISTS "${REALM_PREFIX}/bin/${REALM_CLIENT_HEADER_NAME}")
    message(FATAL_ERROR "Realm client header not found at ${REALM_PREFIX}/bin/${REALM_CLIENT_HEADER_NAME}")
endif()
