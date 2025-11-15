get_filename_component(REALM_CMAKE_DIR "${CMAKE_CURRENT_LIST_FILE}" PATH)
get_filename_component(REALM_PREFIX "${REALM_CMAKE_DIR}/.." ABSOLUTE)

find_package(Threads REQUIRED)

# Determine shared library extension based on platform
if(WIN32)
    set(REALM_LIB_NAME "realm.lib")
else()
    set(REALM_LIB_NAME "realm.a")
endif()

# Define the imported library target
if(NOT TARGET Realm)
    add_library(Realm STATIC IMPORTED)

    # Set the library location
    set_target_properties(Realm PROPERTIES
        IMPORTED_LOCATION "${REALM_PREFIX}/bin/${REALM_LIB_NAME}"
        INTERFACE_INCLUDE_DIRECTORIES "${REALM_PREFIX}/bin"
        INTERFACE_LINK_LIBRARIES "Threads::Threads;${CMAKE_DL_LIBS}"
    )

    # Add platform-specific libraries that Go C archives typically need
    if(UNIX AND NOT APPLE)
        # Linux
        set_property(TARGET Realm APPEND PROPERTY
            INTERFACE_LINK_LIBRARIES "m"
        )
    elseif(APPLE)
        # macOS - may need additional frameworks
        set_property(TARGET Realm APPEND PROPERTY
            INTERFACE_LINK_LIBRARIES "-framework CoreFoundation"
        )
    endif()
endif()

# Set package variables
set(Realm_FOUND TRUE)
set(Realm_LIBRARIES Realm)
set(Realm_INCLUDE_DIRS "${REALM_PREFIX}/bin")

# Verify that the library files exist
if(NOT EXISTS "${REALM_PREFIX}/bin/${REALM_LIB_NAME}")
    message(FATAL_ERROR "Realm library not found at ${REALM_PREFIX}/bin/${REALM_LIB_NAME}")
endif()
if(NOT EXISTS "${REALM_PREFIX}/bin/realm.h")
    message(FATAL_ERROR "Realm header not found at ${REALM_PREFIX}/bin/realm.h")
endif()
